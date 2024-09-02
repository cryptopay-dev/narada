package clients

import (
	"context"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type (
	dbQueryHook struct {
		logger *logrus.Entry
	}

	ctxKey int
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

func NewPostgreSQL(config *viper.Viper, logger *logrus.Logger) *pg.DB {
	config.SetDefault("database.pool", 10)
	config.SetDefault("database.debug", false)

	connection := pg.Connect(&pg.Options{
		Addr:     config.GetString("database.addr"),
		User:     config.GetString("database.user"),
		Password: config.GetString("database.password"),
		Database: config.GetString("database.database"),
		PoolSize: config.GetInt("database.pool"),
	})

	if config.GetBool("database.debug") {
		entry := logger.WithField("module", "db")
		connection.AddQueryHook(dbQueryHook{
			logger: entry,
		})
	}

	return connection
}
