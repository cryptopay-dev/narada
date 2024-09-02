package narada

import (
	"errors"
	"io"
	"testing"
	"time"

	"gopkg.in/h2non/gock.v1"

	"github.com/spf13/viper"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewLogrusSentryHook(t *testing.T) {
	t.Run("It should work with empty sentry", func(t *testing.T) {
		t.Run("Sending error", func(t *testing.T) {
			h := NewLogrusSentryHook()
			err := h.Fire(&logrus.Entry{
				Data: logrus.Fields{
					"error": errors.New("some unknown error"),
				},
			})

			assert.NotNil(t, h)
			assert.NoError(t, err)
		})

		t.Run("Sending message", func(t *testing.T) {
			h := NewLogrusSentryHook()
			err := h.Fire(&logrus.Entry{
				Message: "i am unknown message",
			})

			assert.NotNil(t, h)
			assert.NoError(t, err)
		})
	})

	t.Run("Levels should be only: error, panic, fatal", func(t *testing.T) {
		h := NewLogrusSentryHook()
		assert.NotNil(t, h)
		assert.Equal(t, []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}, h.Levels())
	})
}

func TestNewLogrusSlackHook(t *testing.T) {
	t.Run("Disabled by default", func(t *testing.T) {
		hook := NewLogrusSlackHook(viper.New())
		assert.Nil(t, hook.Fire(nil))
	})

	t.Run("Sending message to Slack", func(t *testing.T) {
		t.Run("With error attached", func(t *testing.T) {
			defer gock.Off()
			done := make(chan bool, 1)

			defer gock.Off()
			gock.New("http://your_token.slack.com").
				Post("/").
				MatchType("json").
				BodyString("").
				ReplyFunc(func(response *gock.Response) {
					done <- true
					response.Status(200)
				})

			cfg := viper.New()
			cfg.Set("logger.slack", true)
			cfg.Set("logger.slack_url", "http://your_token.slack.com")
			cfg.Set("logger.slack_channel", "general")
			cfg.Set("logger.extra", map[string]interface{}{
				"question": 42,
			})
			cfg.Set("app.name", "default")

			logger := logrus.New()
			logger.Out = io.Discard

			hook := NewLogrusSlackHook(cfg)
			logger.AddHook(hook)

			logger.WithError(errors.New("unknown error")).Error("error for slack")

			select {
			case <-done:
			case <-time.After(time.Second * 5):
				t.Fatalf("timeout exceeded")
			}
		})

		t.Run("With error only in message", func(t *testing.T) {
			done := make(chan bool, 1)

			defer gock.Off()
			gock.New("http://your_token.slack.com").
				Post("/").
				MatchType("json").
				BodyString("").
				ReplyFunc(func(response *gock.Response) {
					done <- true
					response.Status(200)
				})

			if gock.HasUnmatchedRequest() {
				t.Fatalf("Unmatched requests from gock")
			}

			cfg := viper.New()
			cfg.Set("logger.slack", true)
			cfg.Set("logger.slack_url", "http://your_token.slack.com")
			cfg.Set("logger.slack_channel", "general")
			cfg.Set("app.name", "default")

			logger := logrus.New()
			logger.Out = io.Discard

			hook := NewLogrusSlackHook(cfg)
			logger.AddHook(hook)

			logger.Errorf("error for slack: %v", errors.New("unknown error"))

			select {
			case <-done:
			case <-time.After(time.Second * 5):
				t.Fatalf("timeout exceeded")
			}
		})
	})
}
