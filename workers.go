package tuktuk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/spf13/viper"

	"github.com/chapsuk/worker"
	"github.com/sirupsen/logrus"
)

type (
	Workers struct {
		logger *logrus.Logger
		config *viper.Viper
		wg     *worker.Group
	}

	Job struct {
		Name        string
		Handler     func()
		Period      time.Duration
		Immediately bool
	}
)

func NewWorkers(logger *logrus.Logger, config *viper.Viper, lc fx.Lifecycle) *Workers {
	w := &Workers{
		wg:     worker.NewGroup(),
		logger: logger,
		config: config,
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

			return nil
		},
	})

	return w
}

func (w Workers) Add(jobs ...Job) {
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

		// Appending handler
		handler := func(ctx context.Context) {
			w.logger.Infof("starting job %s", job.Name)
			defer w.logger.Infof("finished job %s", job.Name)

			job.Handler()
		}

		work := worker.New(handler)
		work.ByTimer(job.Period)
		work.SetImmediately(job.Immediately)

		w.wg.Add(work)
	}
}

func NewWorkerLauncher(wg *Workers, logger *logrus.Logger) {
	logrus.Info("starting workers")
}
