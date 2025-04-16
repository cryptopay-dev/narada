package clients

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type (
	dbQueryHook struct {
		logger *logrus.Entry
	}

	ctxKey int

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

const ctxRequestStartKey ctxKey = 1 + iota

func (d dbQueryHook) BeforeQuery(ctx context.Context, event *pg.QueryEvent) (context.Context, error) {
	return context.WithValue(ctx, ctxRequestStartKey, time.Now()), nil
}

func (d dbQueryHook) AfterQuery(ctx context.Context, event *pg.QueryEvent) error {
	st, ok := ctx.Value(ctxRequestStartKey).(time.Time)
	if !ok {
		return nil
	}

	q, err := event.FormattedQuery()
	if err != nil {
		d.logger.WithError(err).Error("error getting formatted query")
		return nil
	}

	d.logger.WithFields(logrus.Fields{
		"query":   string(q),
		"elapsed": time.Since(st),
	}).Info("query completed")

	return nil
}

func NewPostgreSQL(config *viper.Viper, logger *logrus.Logger) (*pg.DB, error) {
	cfg, err := parseDBConfig(config)
	if err != nil {
		return nil, err
	}

	opts := &pg.Options{
		Addr:     cfg.addr,
		User:     cfg.user,
		Password: cfg.password,
		Database: cfg.database,
		PoolSize: cfg.poolSize,
	}

	if cfg.ssl {
		hp := strings.Split(cfg.addr, ":")
		if len(hp) != 2 {
			return nil, errors.New("database address has wrong format")
		}

		opts.TLSConfig = &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         hp[0],
		}
	}

	connection := pg.Connect(opts)

	if cfg.debug {
		entry := logger.WithField("module", "db")
		connection.AddQueryHook(dbQueryHook{
			logger: entry,
		})
	}

	return connection, nil
}

// NewPostgreSQLForMigrations is a connection that is used for migrations.
// Migrations are implemented with `goose`, which supports only `*sql.DB`.
func NewPostgreSQLForMigrations(config *viper.Viper) (*sql.DB, error) {
	cfg, err := parseDBConfig(config)
	if err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		cfg.user,
		strings.ReplaceAll(url.QueryEscape(cfg.password), ":", "%3A"),
		cfg.addr,
		cfg.database,
	)

	if cfg.ssl {
		dsn += "?sslmode=verify-ca"
	} else {
		dsn += "?sslmode=disable"
	}

	return sql.Open("postgres", dsn)
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
