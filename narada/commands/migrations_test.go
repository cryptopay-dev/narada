package commands

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cryptopay-dev/narada/clients"

	"github.com/pressly/goose"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	// A dedicated ledger name so the test never reads/writes a real goose_db_version table.
	testLedgerTable = "goose_db_version_narada_test"
	testTargetTable = "narada_migration_test"
)

func migrationTestConfig(t *testing.T) *viper.Viper {
	t.Helper()

	addr := os.Getenv("DATABASE_ADDR")
	if addr == "" {
		t.Skip("DATABASE_ADDR not set; skipping migration integration test")
	}

	cfg := viper.New()
	cfg.Set("database.addr", addr)
	cfg.Set("database.user", os.Getenv("DATABASE_USER"))
	cfg.Set("database.password", os.Getenv("DATABASE_PASSWORD"))
	cfg.Set("database.database", os.Getenv("DATABASE_DATABASE"))
	cfg.Set("database.ssl", os.Getenv("DATABASE_SSL"))

	return cfg
}

// isolateLedger points goose at a dedicated ledger, restores the default afterwards, and drops the test tables before and after the test.
func isolateLedger(t *testing.T, v *viper.Viper) {
	t.Helper()

	goose.SetTableName(testLedgerTable)
	t.Cleanup(func() { goose.SetTableName("goose_db_version") })

	dropTestTables(t, v)
	t.Cleanup(func() { dropTestTables(t, v) })
}

// writeTestMigration writes a migration that creates the target table (running extra after it) on up and drops it on down, returning its directory.
func writeTestMigration(t *testing.T, extra string) string {
	t.Helper()

	dir := t.TempDir()
	migration := "-- +goose Up\n" +
		"CREATE TABLE " + testTargetTable + " (id integer PRIMARY KEY);\n" +
		extra +
		"-- +goose Down\n" +
		"DROP TABLE " + testTargetTable + ";\n"
	if err := os.WriteFile(filepath.Join(dir, "00001_create_test_table.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	return dir
}

func tableExists(t *testing.T, v *viper.Viper, name string) bool {
	t.Helper()

	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	var reg *string
	if err := db.QueryRow("SELECT to_regclass($1)", name).Scan(&reg); err != nil {
		t.Fatalf("to_regclass(%s): %v", name, err)
	}

	return reg != nil
}

func dropTestTables(t *testing.T, v *viper.Viper) {
	t.Helper()

	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, q := range []string{
		"DROP TABLE IF EXISTS " + testTargetTable,
		"DROP TABLE IF EXISTS " + testLedgerTable,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("cleanup %q: %v", q, err)
		}
	}
}

func TestMigrateUpDown(t *testing.T) {
	v := migrationTestConfig(t)
	isolateLedger(t, v)
	ctx := context.Background()
	dir := writeTestMigration(t, "")

	if err := migrateUp(ctx, logrus.New(), v, dir); err != nil {
		t.Fatalf("migrateUp: %v", err)
	}
	if !tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateUp, %s should exist", testTargetTable)
	}

	if err := migrateDown(ctx, logrus.New(), v, dir); err != nil {
		t.Fatalf("migrateDown: %v", err)
	}
	if tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateDown, %s should be gone", testTargetTable)
	}
}

// TestMigrateUpConcurrent races several migrators, as ECS does when it starts one init container per task: the advisory lock must serialize them so the migration is applied exactly once.
func TestMigrateUpConcurrent(t *testing.T) {
	v := migrationTestConfig(t)
	isolateLedger(t, v)

	// The sleep widens the window in which an unlocked migrator would also see version 1 as pending.
	dir := writeTestMigration(t, "SELECT pg_sleep(1);\nINSERT INTO "+testTargetTable+" VALUES (1);\n")

	const runners = 5
	errs := make([]error, runners)
	var wg sync.WaitGroup
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = migrateUp(context.Background(), logrus.New(), migrationTestConfig(t), dir)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("runner %d: migrateUp: %v", i, err)
		}
	}

	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, q := range []string{
		"SELECT count(*) FROM " + testTargetTable,
		"SELECT count(*) FROM " + testLedgerTable + " WHERE version_id = 1",
	} {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if n != 1 {
			t.Errorf("%s = %d, want 1", q, n)
		}
	}
}

// TestMigrateUpLockTimeout checks that a migrator gives up once the configured lock timeout elapses while another session holds the lock.
func TestMigrateUpLockTimeout(t *testing.T) {
	v := migrationTestConfig(t)
	v.Set(migrationsLockTimeoutKey, "3s")
	isolateLedger(t, v)
	ctx := context.Background()

	db, err := clients.NewPostgreSQLForMigrations(v)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationsLockID); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationsLockID) }()

	start := time.Now()
	if err := migrateUp(ctx, logrus.New(), v, writeTestMigration(t, "")); err != errMigrationsLockTimeout {
		t.Fatalf("migrateUp = %v, want %v", err, errMigrationsLockTimeout)
	}
	if elapsed := time.Since(start); elapsed < 3*time.Second || elapsed > 10*time.Second {
		t.Errorf("migrateUp waited %s, want about 3s", elapsed)
	}
	if tableExists(t, v, testTargetTable) {
		t.Errorf("%s created despite the lock being held", testTargetTable)
	}
}
