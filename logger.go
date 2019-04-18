package tuktuk

import (
	"fmt"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewLogger(config *viper.Viper) (*logrus.Logger, error) {
	config.SetDefault("logger.formatter", "text")
	config.SetDefault("logger.level", "debug")
	config.SetDefault("logger.catch_errors", true)

	// Setting level
	logger := logrus.New()
	switch lvl := config.GetString("logger.level"); lvl {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		return nil, fmt.Errorf("unknown logger level provided: %s", lvl)
	}

	// Settings formatter
	switch format := config.GetString("logger.formatter"); format {
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{})
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	default:
		return nil, fmt.Errorf("unknown formatter: %s", format)
	}

	// Catching errors with Sentry
	if config.GetBool("logger.catch_errors") {
		client, err := NewSentry(config)
		if err != nil {
			return nil, err
		}

		hook := NewLogrusSentryHook(client)
		logger.AddHook(hook)
	}

	hook := NewLogrusSlackHook(config)
	logger.AddHook(hook)

	return logger, nil
}

func NewNopLogger() *logrus.Logger {
	logger := logrus.New()
	logger.Out = ioutil.Discard

	return logger
}
