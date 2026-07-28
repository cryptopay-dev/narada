package clients

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func NewRedis(config *viper.Viper) (*redis.Client, error) {
	config.SetDefault("redis.pool_size", 10)
	config.SetDefault("redis.idle_timeout", time.Second*60)

	client := redis.NewClient(&redis.Options{
		Addr:            config.GetString("redis.addr"),
		PoolSize:        config.GetInt("redis.pool_size"),
		DB:              config.GetInt("redis.db"),
		ConnMaxIdleTime: config.GetDuration("redis.idle_timeout"),
		Password:        config.GetString("redis.password"),
	})

	if err := client.Ping(context.TODO()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
