package clients

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type dbQueryHook struct {
	logger *logrus.Entry
}

func (d *dbQueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (d *dbQueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	d.logger.WithFields(logrus.Fields{
		"query":   string(event.Query),
		"elapsed": time.Since(event.StartTime),
	}).Info("query completed")
}

func NewPostgreSQL(config *viper.Viper, logger *logrus.Logger) *bun.DB {
	config.SetDefault("database.pool", 10)
	config.SetDefault("database.debug", false)
	config.SetDefault("database.ssl", false)

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s/%s",
		config.GetString("database.user"),
		config.GetString("database.password"),
		config.GetString("database.addr"),
		config.GetString("database.database"),
	)

	if config.GetBool("database.ssl") {
		dsn += "?sslmode=verify-ca"

		if config.GetString("database.path_to_ssl_root_cert") != "" {
			dsn += "&sslrootcert=" + config.GetString("database.path_to_ssl_root_cert")
		}
	} else {
		dsn += "?sslmode=disable"
	}

	pgconn := pgdriver.NewConnector(pgdriver.WithDSN(dsn))

	sqldb := sql.OpenDB(pgconn)
	sqldb.SetMaxOpenConns(config.GetInt("database.pool"))

	db := bun.NewDB(sqldb, pgdialect.New())

	if config.GetBool("database.debug") {
		logger := logger.WithField("module", "db")
		db.AddQueryHook(&dbQueryHook{
			logger: logger,
		})
	}

	return db
}
