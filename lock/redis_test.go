package lock

import (
	"os"
	"testing"
	"time"

	"github.com/cryptopay-dev/narada/clients"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var redisAddr = os.Getenv("REDIS_ADDR")

func TestNewRedisLocker(t *testing.T) {
	cfg := viper.New()
	cfg.Set("redis.addr", redisAddr)
	redis, err := clients.NewRedis(cfg)
	if !assert.NoError(t, err) {
		t.Fail()
	}

	locker := NewRedis(redis)
	assert.NotNil(t, locker)

	mutex := locker.Obtain("test", time.Second)
	{
		locked, err := mutex.Lock()
		assert.True(t, locked)
		assert.NoError(t, err)
	}

	{
		// This lock extending should be true
		locked, err := mutex.Lock()
		assert.True(t, locked)
		assert.NoError(t, err)
	}

	mutex1 := locker.Obtain("test", time.Second)
	locked, err := mutex1.Lock()
	assert.False(t, locked)
	assert.NoError(t, err)
}
