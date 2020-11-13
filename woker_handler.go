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
	logger *logrus.Entry

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
		logger: logger.WithField("job_name", job.Name),
	}

	go jh.refreshExclusiveLock()

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
			if err != nil {
				jh.logger.WithError(err).Error("error obtaining lock")

				jh.lock = nil
				return
			}
			if !obtained {
				jh.logger.Debug("exclusive lock are already obtained, skipping")

				jh.lock = nil
				return
			}

			jh.lock = mutex
		}

		jh.logger.Infof("job started")
		defer func(start time.Time) {
			jh.logger.WithField("duration", time.Since(start).Seconds()).Infof("job finished")
		}(time.Now())

		job.Handler()
	}

	return jh
}

func (j *jobHandler) refreshExclusiveLock() {
	if !j.job.Exclusive {
		return
	}

	for range time.Tick(time.Second * 30) {
		if j.lock == nil {
			continue
		}

		if _, err := j.lock.Lock(); err != nil {
			j.logger.WithError(err).Error("error refreshing lock")
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
