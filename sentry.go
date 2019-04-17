package tuktuk

import (
	"github.com/getsentry/raven-go"
	"github.com/spf13/viper"
)

func NewSentry(config *viper.Viper) (*raven.Client, error) {
	return raven.New(config.GetString("sentry.dsn"))
}
