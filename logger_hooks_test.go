package tuktuk

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewLogrusSentryHook(t *testing.T) {
	t.Run("It should work with empty sentry", func(t *testing.T) {
		t.Run("Sending error", func(t *testing.T) {
			h := NewLogrusSentryHook(nil)
			err := h.Fire(&logrus.Entry{
				Data: logrus.Fields{
					"error": errors.New("some unknown error"),
				},
			})

			assert.NotNil(t, h)
			assert.NoError(t, err)
		})

		t.Run("Sending message", func(t *testing.T) {
			h := NewLogrusSentryHook(nil)
			err := h.Fire(&logrus.Entry{
				Message: "i am unknown message",
			})

			assert.NotNil(t, h)
			assert.NoError(t, err)
		})
	})

	t.Run("Levels should be only: error, panic, fatal", func(t *testing.T) {
		h := NewLogrusSentryHook(nil)
		assert.NotNil(t, h)
		assert.Equal(t, []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}, h.Levels())
	})
}

func TestNewLogrusSlackHook(t *testing.T) {

}
