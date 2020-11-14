package clients

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var redisAddr = os.Getenv("REDIS_ADDR")

func TestNewRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("Successful connect", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("redis.addr", redisAddr)

		r, err := NewRedis(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})

	t.Run("Error while connecting", func(t *testing.T) {
		cfg := viper.New()
		cfg.Set("redis.addr", "127.0.0.1:8000")

		r, err := NewRedis(cfg)
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}
