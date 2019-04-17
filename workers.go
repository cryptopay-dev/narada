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

	WorkerParams struct {
		fx.In

		Logger *logrus.Logger
		Config *viper.Viper
		Lc     fx.Lifecycle
		Jobs   []Job `group:"jobs"`
	}

	Job struct {
		Name        string
		Handler     func()
		Period      time.Duration
		Immediately bool
	}

	JobResult struct {
		fx.Out

		Job Job `group:"jobs"`
	}
)

func NewWorkers(opts WorkerParams) *Workers {
	w := &Workers{
		wg:     worker.NewGroup(),
		logger: opts.Logger,
		config: opts.Config,
	}

	for _, job := range opts.Jobs {
		opts.Logger.WithField("job", job).Info("adding new job to workers")

		name := strings.ToLower(job.Name)
		// Reading configuration
		if opts.Config.IsSet(fmt.Sprintf("jobs.%s", name)) {
			enabledKey := fmt.Sprintf("jobs.%s.enabled", name)
			periodKey := fmt.Sprintf("jobs.%s.period", name)

			opts.Config.SetDefault(enabledKey, true)
			opts.Config.SetDefault(periodKey, job.Period)

			if !opts.Config.GetBool(enabledKey) {
				opts.Logger.Infof("skipping %s job, it's disabled by configuration", name)
				continue
			}

			job.Period = opts.Config.GetDuration(periodKey)
		}

		// Appending handler
		handler := func(ctx context.Context) {
			opts.Logger.Infof("starting job %s", job.Name)
			defer opts.Logger.Infof("finished job %s", job.Name)

			job.Handler()
		}

		work := worker.New(handler)
		work.ByTimer(job.Period)
		work.SetImmediately(job.Immediately)

		w.wg.Add(work)
	}

	opts.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			opts.Logger.Info("starting jobs")
			w.wg.Run()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			opts.Logger.Info("stopping jobs")
			w.wg.Stop()

			return nil
		},
	})

	return w
}

func NewJob(job Job) JobResult {
	return JobResult{
		Job: job,
	}
}

func NewWorkerLauncher(wg *Workers, logger *logrus.Logger) {
	logrus.Info("starting workers")
}
