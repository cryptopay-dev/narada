package lib

import (
	"time"

	lock "github.com/bsm/redis-lock"
	"github.com/go-redis/redis"
)

type (
	Locker interface {
		Obtain(name string, expire time.Duration) Mutex
	}

	Mutex interface {
		Lock() (bool, error)
		Unlock() error
	}

	RedisLock struct {
		redis *redis.Client
	}
)

func NewRedisLocker(redis *redis.Client) Locker {
	return &RedisLock{
		redis: redis,
	}
}

func (rl *RedisLock) Obtain(name string, expire time.Duration) Mutex {
	return lock.New(rl.redis, name, &lock.Options{
		LockTimeout: expire,
		RetryCount:  3,
		RetryDelay:  time.Millisecond * 100,
	})
}
