package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chapsuk/worker"
	"github.com/cryptopay-dev/narada/v2/lock"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Workers struct {
		locker   lock.Locker
		logger   *slog.Logger
		config   *viper.Viper
		handlers map[string]*handler
		wg       *worker.Group
	}

	Options struct {
		fx.In

		Logger *slog.Logger
		Config *viper.Viper
		Locker lock.Locker
		LC     fx.Lifecycle
	}

	Job struct {
		Name             string
		Handler          func(ctx context.Context)
		Period           time.Duration
		Cron             string
		Exclusive        bool
		ExclusiveTimeout time.Duration
		Immediately      bool
	}
)

func New(opts Options) (*Workers, error) {
	w := &Workers{
		wg:       worker.NewGroup(),
		logger:   opts.Logger.With("module", "workers"),
		locker:   opts.Locker,
		config:   opts.Config,
		handlers: make(map[string]*handler),
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
						w.logger.Error("error releasing lock", "job_name", name, slog.Any("error", err))
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
				w.logger.Info("skipping job, it's disabled by configuration", "job_name", name)
				continue
			}

			job.Period = w.config.GetDuration(periodKey)
		}

		w.logger.Info("adding new job to workers",
			"job_name", name,
			"job_period", job.Period,
			"job_cron", job.Cron,
		)

		func(j Job) {
			// Creating handler
			jh := newHandler(job, w.locker, w.logger)

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
