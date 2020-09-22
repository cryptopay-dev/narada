package narada

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/cryptopay-dev/narada/lib"
)

type jobHandler struct {
	job    Job
	locker lib.Locker
	logger *logrus.Logger

	lock    lib.Mutex
	handler func(ctx context.Context)
}

func newJobHandler(
	job Job,
	locker lib.Locker,
	logger *logrus.Entry,
) *jobHandler {
	jh := &jobHandler{
		job:    job,
		locker: locker,
	}

	go jh.refreshExclusiveLock()
	entry := logger.WithField("job_name", job.Name)

	jh.handler = func(ctx context.Context) {
		// Checking if it's exclusive
		if job.Exclusive {
			lockKey := strings.Join([]string{Prefix, "workers", job.Name, "exclusive"}, ":")

			var mutex lib.Mutex
			if jh.lock == nil {
				mutex = locker.Obtain(lockKey, time.Minute)
			} else {
				mutex = jh.lock
			}

			// Trying to lock, if cannot we should wait till next run
			obtained, err := mutex.Lock()
			if !obtained || err != nil {
				entry.Debugf("exclusive lock are already obtained, skipping")

				jh.lock = nil
				return
			}

			jh.lock = mutex
		}

		entry.Infof("starting job")
		t := time.Now()
		defer entry.Infof("finished job in %s", time.Since(t))

		job.Handler()
	}

	return jh
}

func (j *jobHandler) refreshExclusiveLock() {
	if !j.job.Exclusive {
		return
	}

	for range time.Tick(time.Second * 30) {
		if j.lock != nil {
			_, err := j.lock.Lock()
			if err != nil {
				j.logger.WithError(err).Errorf("error refreshing lock on job: %s", j.job.Name)
			}
		}
	}
}

func (j *jobHandler) Handler() func(ctx context.Context) {
	return j.handler
}

func (j *jobHandler) ReleaseLocks() error {
	if j.lock != nil {
		return j.lock.Unlock()
	}

	return nil
}
