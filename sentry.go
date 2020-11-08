package narada

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func NewSentryInvoke(config *viper.Viper, lc fx.Lifecycle) error {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return sentry.Init(sentry.ClientOptions{
				Dsn:         config.GetString("sentry.dsn"),
				Environment: config.GetString("sentry.environment"),
			})
		},
		OnStop: func(ctx context.Context) error {
			sentry.Flush(10 * time.Second)
			return nil
		},
	})

	return nil
}
