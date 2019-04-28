package tuktuk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m1ome/tuktuk/lib"

	"go.uber.org/fx"

	"github.com/spf13/viper"

	"github.com/chapsuk/worker"
	"github.com/sirupsen/logrus"
)

type (
	Workers struct {
		locker   lib.Locker
		logger   *logrus.Entry
		config   *viper.Viper
		handlers map[string]*jobHandler
		wg       *worker.Group
	}

	Job struct {
		Name             string
		Handler          func()
		Period           time.Duration
		Exclusive        bool
		ExclusiveTimeout time.Duration
		Immediately      bool
	}
)

func NewWorkers(
	logger *logrus.Logger,
	locker lib.Locker,
	config *viper.Viper,
	lc fx.Lifecycle,
) *Workers {
	w := &Workers{
		wg:       worker.NewGroup(),
		logger:   logger.WithField("module", "workers"),
		locker:   locker,
		config:   config,
		handlers: make(map[string]*jobHandler),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting jobs")
			w.wg.Run()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("stopping jobs")
			w.wg.Stop()

			if len(w.handlers) > 0 {
				logger.Info("releasing locks from handlers")

				for name, handler := range w.handlers {
					if err := handler.ReleaseLocks(); err != nil {
						w.logger.WithError(err).WithField("job_name", name).Error("error releasing lock")
					}
				}
			}

			return nil
		},
	})

	return w
}

func (w *Workers) Add(jobs ...Job) {
	for _, job := range jobs {
		w.logger.WithField("job", job).Info("adding new job to workers")
		name := strings.ToLower(job.Name)

		// Reading configuration
		if w.config.IsSet(fmt.Sprintf("jobs.%s", name)) {
			enabledKey := fmt.Sprintf("jobs.%s.enabled", name)
			periodKey := fmt.Sprintf("jobs.%s.period", name)

			w.config.SetDefault(enabledKey, true)
			w.config.SetDefault(periodKey, job.Period)

			if !w.config.GetBool(enabledKey) {
				w.logger.Infof("skipping %s job, it's disabled by configuration", name)
				continue
			}

			job.Period = w.config.GetDuration(periodKey)
		}

		func(j Job) {
			// Creating handler
			jh := newJobHandler(job, w.locker, w.logger)

			// Appending job
			work := worker.New(jh.Handler())
			work.ByTimer(j.Period)
			work.SetImmediately(j.Immediately)

			w.wg.Add(work)

			// Adding to handlers
			w.handlers[j.Name] = jh
		}(job)
	}
}
