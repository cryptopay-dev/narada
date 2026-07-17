package clients

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestNewPostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	cfg := setupConfig()
	logger := logrus.New()

	db, err := NewPostgreSQL(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	type StringResult struct {
		Message string
	}
	var res StringResult
	_, err = db.QueryOne(&res, "SELECT 'hello' AS message")
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != "hello" {
		t.Error("unexpected message")
	}
}

func TestNewPostgreSQLForMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	cfg := setupConfig()

	db, err := NewPostgreSQLForMigrations(cfg)
	if err != nil {
		t.Fatal(err)
	}

	type StringResult struct {
		Message string
	}
	var res StringResult
	err = db.QueryRow("SELECT 'hello' AS message").Scan(&res.Message)
	if err != nil {
		t.Fatal(err)
	}

	if res.Message != "hello" {
		t.Error("unexpected message")
	}
}

func setupConfig() *viper.Viper {
	cfg := viper.New()
	cfg.Set("database.addr", os.Getenv("DATABASE_ADDR"))
	cfg.Set("database.user", os.Getenv("DATABASE_USER"))
	cfg.Set("database.password", os.Getenv("DATABASE_PASSWORD"))
	cfg.Set("database.database", os.Getenv("DATABASE_DATABASE"))
	cfg.Set("database.ssl", os.Getenv("DATABASE_SSL"))

	return cfg
}

func TestNewPostgreSQLOptions(t *testing.T) {
	logger := logrus.New()

	// Missing address is rejected.
	if _, err := NewPostgreSQL(viper.New(), logger); err == nil {
		t.Error("expected error for missing database address")
	}

	// SSL enabled with an address that has no host:port split is rejected.
	badAddr := viper.New()
	badAddr.Set("database.addr", "no-port-here")
	badAddr.Set("database.ssl", true)
	if _, err := NewPostgreSQL(badAddr, logger); err == nil {
		t.Error("expected error for malformed address with ssl enabled")
	}

	// SSL enabled with a valid address builds a TLS-configured connection.
	sslCfg := viper.New()
	sslCfg.Set("database.addr", "localhost:5432")
	sslCfg.Set("database.user", "u")
	sslCfg.Set("database.database", "d")
	sslCfg.Set("database.ssl", true)
	db, err := NewPostgreSQL(sslCfg, logger)
	if err != nil {
		t.Fatalf("ssl NewPostgreSQL: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
	_ = db.Close()
}

func TestNewPostgreSQLForMigrationsOptions(t *testing.T) {
	// Missing address is rejected.
	if _, err := NewPostgreSQLForMigrations(viper.New()); err == nil {
		t.Error("expected error for missing database address")
	}

	// SSL enabled produces a verify-ca DSN (lazy open, no connection made).
	sslCfg := viper.New()
	sslCfg.Set("database.addr", "localhost:5432")
	sslCfg.Set("database.user", "u")
	sslCfg.Set("database.password", "p")
	sslCfg.Set("database.database", "d")
	sslCfg.Set("database.ssl", true)
	db, err := NewPostgreSQLForMigrations(sslCfg)
	if err != nil {
		t.Fatalf("ssl NewPostgreSQLForMigrations: %v", err)
	}
	_ = db.Close()
}

// Exercises the debug query-hook path (BeforeQuery/AfterQuery) against a real DB.
func TestNewPostgreSQLDebugHooks(t *testing.T) {
	if testing.Short() || os.Getenv("DATABASE_ADDR") == "" {
		t.Skip("no database configured")
	}

	cfg := setupConfig()
	cfg.Set("database.debug", true)

	db, err := NewPostgreSQL(cfg, logrus.New())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	var res struct{ Message string }
	if _, err := db.QueryOne(&res, "SELECT 'hi' AS message"); err != nil {
		t.Fatal(err)
	}
	if res.Message != "hi" {
		t.Error("unexpected message")
	}
}
