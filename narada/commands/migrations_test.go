package commands

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cryptopay-dev/narada/v2/clients"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
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

// isolatedLedger points goose at a dedicated ledger so tests never touch a real goose_db_version.
var isolatedLedger = goose.WithTableName(testLedgerTable)

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

// TestMigrateUpDown drives the real goose up/down path against Postgres, so any
// future goose bump that breaks migration application or rollback fails here.
func TestMigrateUpDown(t *testing.T) {
	v := migrationTestConfig(t)
	logger := slog.New(slog.DiscardHandler)
	ctx := context.Background()

	// Clean slate before, guaranteed teardown after.
	dropTestTables(t, v)
	t.Cleanup(func() { dropTestTables(t, v) })

	dir := writeTestMigration(t, "")

	// Up: target absent before, present after; goose records it in the isolated ledger.
	if tableExists(t, v, testTargetTable) {
		t.Fatalf("precondition failed: %s already exists", testTargetTable)
	}
	if err := migrateUp(ctx, logger, v, dir, isolatedLedger); err != nil {
		t.Fatalf("migrateUp: %v", err)
	}
	if !tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateUp, %s should exist", testTargetTable)
	}
	if !tableExists(t, v, testLedgerTable) {
		t.Fatalf("after migrateUp, ledger %s should exist", testLedgerTable)
	}

	// Down: target gone again.
	if err := migrateDown(ctx, logger, v, dir, isolatedLedger); err != nil {
		t.Fatalf("migrateDown: %v", err)
	}
	if tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateDown, %s should be gone", testTargetTable)
	}
}

// TestMigrateUpConcurrent races several migrators, as ECS does when it starts one init container per task: the advisory lock must serialize them so the migration is applied exactly once.
func TestMigrateUpConcurrent(t *testing.T) {
	v := migrationTestConfig(t)
	logger := slog.New(slog.DiscardHandler)

	dropTestTables(t, v)
	t.Cleanup(func() { dropTestTables(t, v) })

	// The sleep widens the window in which an unlocked migrator would also see version 1 as pending.
	dir := writeTestMigration(t, "SELECT pg_sleep(1);\nINSERT INTO "+testTargetTable+" VALUES (1);\n")

	const runners = 5
	errs := make([]error, runners)
	var wg sync.WaitGroup
	for i := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = migrateUp(context.Background(), logger, migrationTestConfig(t), dir, isolatedLedger)
		}()
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
	v.Set(migrationsLockTimeoutKey, "5s")
	logger := slog.New(slog.DiscardHandler)
	ctx := context.Background()

	dropTestTables(t, v)
	t.Cleanup(func() { dropTestTables(t, v) })

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

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatalf("hold lock: %v", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lock.DefaultLockID) }()

	start := time.Now()
	err = migrateUp(ctx, logger, v, writeTestMigration(t, ""), isolatedLedger)
	if err == nil {
		t.Fatal("migrateUp succeeded while the lock was held elsewhere")
	}
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Errorf("migrateUp waited %s, want about 5s", elapsed)
	}
	if tableExists(t, v, testTargetTable) {
		t.Errorf("%s created despite the lock being held", testTargetTable)
	}
	t.Logf("lock timeout error: %v", err)
}

// TestMigrateCreate covers the scaffolding path (no DB needed — it only writes a file).
func TestMigrateCreate(t *testing.T) {
	dir := t.TempDir()

	if err := migrateCreate(slog.New(slog.DiscardHandler), dir, "add_widget", DefaultMigrationsType); err != nil {
		t.Fatalf("migrateCreate: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*_add_widget.sql"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one migration file, got %d: %v", len(matches), matches)
	}
}
