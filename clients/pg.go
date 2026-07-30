package clients

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type (
	dbQueryHook struct {
		logger *slog.Logger
	}

	dbConfig struct {
		addr     string
		user     string
		password string
		database string
		poolSize int
		ssl      bool
		debug    bool
	}
)

var _ bun.QueryHook = (*dbQueryHook)(nil)

func (d *dbQueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (d *dbQueryHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	if event.Err != nil {
		d.logger.Error("query failed",
			"query", event.Query,
			"elapsed", time.Since(event.StartTime),
			slog.Any("error", event.Err),
		)

		return
	}

	d.logger.Info("query completed",
		"query", event.Query,
		"elapsed", time.Since(event.StartTime),
	)
}

// NewPostgreSQL builds the application's Postgres handle.
func NewPostgreSQL(config *viper.Viper, logger *slog.Logger) (*bun.DB, error) {
	cfg, err := parseDBConfig(config)
	if err != nil {
		return nil, err
	}

	sqldb, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	db := bun.NewDB(sqldb, pgdialect.New())

	if cfg.debug {
		db.AddQueryHook(&dbQueryHook{
			logger: logger.With("module", "db"),
		})
	}

	return db, nil
}

// NewPostgreSQLForMigrations is a connection that is used for migrations.
// Migrations are implemented with `goose`, which supports only `*sql.DB`.
func NewPostgreSQLForMigrations(config *viper.Viper) (*sql.DB, error) {
	cfg, err := parseDBConfig(config)
	if err != nil {
		return nil, err
	}

	return openDB(cfg)
}

// openDB builds a lazily-connecting *sql.DB from cfg. Both the bun handle and
// the migrations handle go through here, so they share one TLS policy.
func openDB(cfg dbConfig) (*sql.DB, error) {
	// pgdriver panics on an empty user/database rather than returning an error,
	// so leave those options off entirely when unset and let it apply its own
	// defaults — which is what go-pg did before.
	opts := []pgdriver.Option{
		pgdriver.WithAddr(cfg.addr),
		pgdriver.WithPassword(cfg.password),
	}

	if cfg.user != "" {
		opts = append(opts, pgdriver.WithUser(cfg.user))
	}

	if cfg.database != "" {
		opts = append(opts, pgdriver.WithDatabase(cfg.database))
	}

	if cfg.ssl {
		// pgdriver derives no ServerName of its own when handed a tls.Config, so
		// take the host from the address the same way the DSN parser would.
		host, _, err := net.SplitHostPort(cfg.addr)
		if err != nil {
			return nil, errors.New("database address has wrong format")
		}

		opts = append(opts, pgdriver.WithTLSConfig(&tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}))
	} else {
		// pgdriver defaults to TLS-with-skip-verify; turn it off explicitly.
		opts = append(opts, pgdriver.WithInsecure(true))
	}

	sqldb := sql.OpenDB(pgdriver.NewConnector(opts...))
	sqldb.SetMaxOpenConns(cfg.poolSize)
	sqldb.SetMaxIdleConns(cfg.poolSize)

	return sqldb, nil
}

func parseDBConfig(config *viper.Viper) (dbConfig, error) {
	config.SetDefault("database.pool", 10)
	config.SetDefault("database.debug", false)
	config.SetDefault("database.ssl", false)

	dbAddr := config.GetString("database.addr")
	if dbAddr == "" {
		return dbConfig{}, errors.New("missing database address")
	}

	return dbConfig{
		addr:     dbAddr,
		user:     config.GetString("database.user"),
		password: config.GetString("database.password"),
		database: config.GetString("database.database"),
		poolSize: config.GetInt("database.pool"),
		ssl:      config.GetBool("database.ssl"),
		debug:    config.GetBool("database.debug"),
	}, nil
}
