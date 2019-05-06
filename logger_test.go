package narada

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"
)

func TestNewNopLogger(t *testing.T) {
	nop := NewNopLogger()
	assert.NotNil(t, nop)
}

func TestNewLogger(t *testing.T) {
	t.Run("Logger without a config", func(t *testing.T) {
		logger, err := NewLogger(viper.New())
		assert.NotNil(t, logger)
		assert.NoError(t, err)
	})

	t.Run("Logger with different levels", func(t *testing.T) {
		tests := []struct {
			level       string
			expectation string
		}{
			{
				"debug",
				"DEBUG",
			},
			{
				"info",
				"DEBUG",
			},
			{
				"warn",
				"WARN",
			},
			{
				"error",
				"ERROR",
			},
		}

		for _, test := range tests {
			cfg := viper.New()
			cfg.Set("logger.level", test.level)

			logger, err := NewLogger(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, logger)
		}
	})

	t.Run("Logger with different formatter", func(t *testing.T) {
		tests := []struct {
			formatter   string
			expectation string
		}{
			{
				"json",
				"{",
			},
			{
				"text",
				"INFO",
			},
		}

		for _, test := range tests {
			cfg := viper.New()
			cfg.Set("logger.formatter", test.formatter)

			logger, err := NewLogger(cfg)
			assert.NoError(t, err)
			assert.NotNil(t, logger)
		}
	})

	t.Run("Logger with unknown level", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("logger.level", "unknown_level")

		logger, err := NewLogger(cfg)
		assert.Error(t, err)
		assert.Nil(t, logger)
	})

	t.Run("Logger with unknown formatter", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("logger.formatter", "unknown formatter")

		logger, err := NewLogger(cfg)
		assert.Error(t, err)
		assert.Nil(t, logger)
	})

	t.Run("Logger with bad sentry DSN", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("sentry.dsn", "unknown_dsn")

		logger, err := NewLogger(cfg)
		assert.Error(t, err)
		assert.Nil(t, logger)
	})

}
