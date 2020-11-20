package lock

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type (
	RedisLocker struct {
		locker *redislock.Client
	}

	redisMutex struct {
		locker *redislock.Client
		lock   *redislock.Lock
		name   string
		expire time.Duration
	}
)

var (
	ctx = context.TODO()

	lopts = &redislock.Options{
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(time.Millisecond*100), 3),
	}
)

func NewRedis(redis *redis.Client) Locker {
	return &RedisLocker{
		locker: redislock.New(redis),
	}
}

func (rl *RedisLocker) Obtain(name string, expire time.Duration) Mutex {
	return &redisMutex{
		locker: rl.locker,
		name:   name,
		expire: expire,
	}
}

func (mu *redisMutex) Lock() (bool, error) {
	if mu.lock != nil {
		if err := mu.lock.Refresh(ctx, mu.expire, lopts); err != nil {
			return false, errors.Wrap(err, "refresh lock")
		}
		return true, nil
	}

	lock, err := mu.locker.Obtain(ctx, mu.name, mu.expire, lopts)
	if err == redislock.ErrNotObtained {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	mu.lock = lock

	return true, nil
}

func (mu *redisMutex) Unlock() error {
	if mu.lock == nil {
		return errLockNotHeld
	}

	return mu.lock.Release(ctx)
}
