package clients

import (
	"github.com/go-pg/pg"
	"github.com/spf13/viper"
)

func NewPostgreSQL(config *viper.Viper) *pg.DB {
	config.SetDefault("database.pool", 10)

	return pg.Connect(&pg.Options{
		Addr:     config.GetString("database.addr"),
		User:     config.GetString("database.user"),
		Password: config.GetString("database.password"),
		Database: config.GetString("database.database"),
		PoolSize: config.GetInt("database.pool"),
	})
}
