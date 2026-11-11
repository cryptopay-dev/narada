package narada

import (
	"bytes"
	"strings"
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

	t.Run("Logger writes lower-case levels", func(t *testing.T) {
		tests := []struct {
			formatter string
			expected  []string
		}{
			{
				"text",
				[]string{"level=debug", "level=info", "level=warning", "level=error", "data.level=custom"},
			},
			{
				"json",
				[]string{`"level":"debug"`, `"level":"info"`, `"level":"warning"`, `"level":"error"`, `"data":{"level":"custom"}`},
			},
		}

		for _, test := range tests {
			cfg := viper.New()
			cfg.Set("logger.formatter", test.formatter)
			cfg.Set("logger.catch_errors", false)

			var out bytes.Buffer
			logger, err := newLogger(cfg, &out)
			assert.NoError(t, err)

			logger.Debug("message")
			logger.Info("message")
			logger.Warn("message")
			logger.Error("message")
			logger.WithGroup("data").Info("message", "level", "custom")

			t.Log("\n" + out.String())

			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			assert.Len(t, lines, len(test.expected))
			for i, expected := range test.expected {
				assert.Contains(t, lines[i], expected)
			}
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
}
