package lock

import (
	"errors"
	"time"
)

type (
	Locker interface {
		Obtain(name string, expire time.Duration) Mutex
	}

	Mutex interface {
		Lock() (bool, error)
		Unlock() error
	}
)

var (
	errLockNotHeld = errors.New("lock: lock not held")
)
