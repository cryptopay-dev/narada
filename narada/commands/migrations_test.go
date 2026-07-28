package commands

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/cryptopay-dev/narada/v2/clients"

	"github.com/pressly/goose/v3"
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

	// Isolate goose's ledger, and restore the default afterwards.
	goose.SetTableName(testLedgerTable)
	t.Cleanup(func() { goose.SetTableName("goose_db_version") })

	// Clean slate before, guaranteed teardown after.
	dropTestTables(t, v)
	t.Cleanup(func() { dropTestTables(t, v) })

	// A temp migration that creates the target table on up and drops it on down.
	dir := t.TempDir()
	migration := "-- +goose Up\n" +
		"CREATE TABLE " + testTargetTable + " (id integer PRIMARY KEY);\n" +
		"-- +goose Down\n" +
		"DROP TABLE " + testTargetTable + ";\n"
	if err := os.WriteFile(filepath.Join(dir, "00001_create_test_table.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	// Up: target absent before, present after; goose records it in the isolated ledger.
	if tableExists(t, v, testTargetTable) {
		t.Fatalf("precondition failed: %s already exists", testTargetTable)
	}
	if err := migrateUp(logger, v, dir); err != nil {
		t.Fatalf("migrateUp: %v", err)
	}
	if !tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateUp, %s should exist", testTargetTable)
	}
	if !tableExists(t, v, testLedgerTable) {
		t.Fatalf("after migrateUp, ledger %s should exist", testLedgerTable)
	}

	// Down: target gone again.
	if err := migrateDown(logger, v, dir); err != nil {
		t.Fatalf("migrateDown: %v", err)
	}
	if tableExists(t, v, testTargetTable) {
		t.Fatalf("after migrateDown, %s should be gone", testTargetTable)
	}
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
