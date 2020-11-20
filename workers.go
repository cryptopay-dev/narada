package narada

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chapsuk/worker"
	"github.com/cryptopay-dev/narada/lock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Workers struct {
		locker   lock.Locker
		logger   *logrus.Entry
		config   *viper.Viper
		handlers map[string]*jobHandler
		wg       *worker.Group
	}

	WorkersOptions struct {
		fx.In

		Logger *logrus.Logger
		Config *viper.Viper
		Locker lock.Locker
		LC     fx.Lifecycle
	}

	Job struct {
		Name             string
		Handler          func()
		Period           time.Duration
		Cron             string
		Exclusive        bool
		ExclusiveTimeout time.Duration
		Immediately      bool
	}
)

func NewWorkers(opts WorkersOptions) (*Workers, error) {
	w := &Workers{
		wg:       worker.NewGroup(),
		logger:   opts.Logger.WithField("module", "workers"),
		locker:   opts.Locker,
		config:   opts.Config,
		handlers: make(map[string]*jobHandler),
	}

	opts.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			w.logger.Info("starting jobs")
			w.wg.Run()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			w.logger.Info("stopping jobs")
			w.wg.Stop()

			if len(w.handlers) > 0 {
				w.logger.Info("releasing locks from handlers")

				for name, handler := range w.handlers {
					if err := handler.ReleaseLocks(); err != nil {
						w.logger.WithError(err).WithField("job_name", name).Error("error releasing lock")
					}
				}
			}

			return nil
		},
	})

	return w, nil
}

func (w *Workers) Add(jobs ...Job) {
	for _, job := range jobs {
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

		w.logger.WithFields(logrus.Fields{
			"job_name":   name,
			"job_period": job.Period,
			"job_cron":   job.Cron,
		}).Info("adding new job to workers")

		func(j Job) {
			// Creating handler
			jh := newJobHandler(job, w.locker, w.logger)

			// Appending job
			work := worker.New(jh.Handler())

			if j.Period != 0 {
				work.ByTimer(j.Period)
			}

			if j.Cron != "" {
				work.ByCronSpec(j.Cron)
			}

			work.SetImmediately(j.Immediately)

			w.wg.Add(work)

			// Adding to handlers
			w.handlers[j.Name] = jh
		}(job)
	}
}
