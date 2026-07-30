package narada

import (
	"context"
	"log/slog"

	"github.com/cryptopay-dev/narada/v2/clients"
	"github.com/cryptopay-dev/narada/v2/lock"
	"github.com/cryptopay-dev/narada/v2/worker"
	"github.com/spf13/viper"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

type (
	Narada struct {
		providers []interface{}
		logger    *slog.Logger
		config    *viper.Viper
		app       *fx.App
	}

	Options struct {
		Name      string
		Version   string
		EnvPrefix string
	}
)

func (t Narada) HandleError(err error) {
	Fatal(t.logger, err.Error())
}

func New(opts Options, providers ...interface{}) *Narada {
	config, err := NewConfig(opts.EnvPrefix)
	if err != nil {
		logger, _ := NewLogger(viper.New())
		Fatal(logger, "error reading configuration", Err(err))
	}

	config.SetDefault("app.name", opts.Name)
	config.SetDefault("app.version", opts.Version)

	logger, err := NewLogger(config)
	if err != nil {
		logger, _ := NewLogger(viper.New())
		Fatal(logger, "error creating logger from configuration", Err(err))
	}

	return &Narada{
		providers: providers,
		logger:    logger,
		config:    config,
	}
}

func (t *Narada) Start(fn interface{}) {
	t.app = t.build(
		fx.Invoke(
			// Adding servers by default
			NewMetricsInvoke,
			NewProfilerInvoke,
			NewSentryInvoke,

			// Invoke user-defined function
			fn,
		),
	)

	t.app.Run()
}

func (t *Narada) Stop() {
	err := t.app.Stop(context.Background())
	if err != nil {
		Fatal(t.logger, "error stopping", Err(err))
	}
}

func (t *Narada) Invoke(fn interface{}) {
	t.build(
		fx.Invoke(fn),
	)
}

func (t *Narada) build(opts ...fx.Option) *fx.App {
	// Creating application
	opts = append(opts,
		// Silencing fx's own lifecycle logging
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),

		fx.ErrorHook(t),

		fx.Provide(
			// Fundamentals
			func() *slog.Logger { return t.logger },
			func() *viper.Viper { return t.config },

			// Servers handling
			NewMultiServers,

			// Workers handling
			lock.NewRedis,
			worker.New,

			// Clients
			clients.NewPostgreSQL,
			clients.NewRedis,
		),

		fx.Provide(t.providers...),
	)

	return fx.New(opts...)
}
