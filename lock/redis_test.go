package lock

import (
	"os"
	"testing"
	"time"

	"github.com/cryptopay-dev/narada/v2/clients"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

var redisAddr = os.Getenv("REDIS_ADDR")

func TestNewRedisLocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

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

func TestRedisMutexUnlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	cfg := viper.New()
	cfg.Set("redis.addr", redisAddr)
	redis, err := clients.NewRedis(cfg)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	locker := NewRedis(redis)

	// Unlocking a mutex that was never locked reports lock-not-held.
	mu := locker.Obtain("unlock-test", time.Second)
	assert.ErrorIs(t, mu.Unlock(), errLockNotHeld)

	// Lock, then Unlock releases cleanly.
	locked, err := mu.Lock()
	assert.True(t, locked)
	assert.NoError(t, err)
	assert.NoError(t, mu.Unlock())

	// After release the key is free, so a fresh mutex can obtain it.
	mu2 := locker.Obtain("unlock-test", time.Second)
	locked, err = mu2.Lock()
	assert.True(t, locked)
	assert.NoError(t, err)
	assert.NoError(t, mu2.Unlock())
}
