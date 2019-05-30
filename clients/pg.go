package clients

import (
	"time"

	"github.com/go-pg/pg"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

type dbQueryHook struct {
	logger *logrus.Entry
}

func (d dbQueryHook) BeforeQuery(event *pg.QueryEvent) {
	event.Ctx = context.WithValue(event.Ctx, "start_time", time.Now())
}

func (d dbQueryHook) AfterQuery(event *pg.QueryEvent) {
	st, ok := event.Ctx.Value("start_time").(time.Time)
	if ok {
		q, err := event.FormattedQuery()
		if err != nil {
			d.logger.WithError(err).Error("error getting formatted query")
			return
		}

		d.logger.WithFields(logrus.Fields{
			"query":   q,
			"elapsed": time.Since(st),
		})
	}
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
