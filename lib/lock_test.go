package lib

import (
	"testing"
	"time"

	"github.com/m1ome/narada/clients"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewRedisLocker(t *testing.T) {
	cfg := viper.New()
	cfg.Set("redis.addr", "127.0.0.1:6379")
	redis, err := clients.NewRedis(cfg)
	if !assert.NoError(t, err) {
		t.Fail()
	}

	locker := NewRedisLocker(redis)
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
