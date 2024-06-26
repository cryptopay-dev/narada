package narada

import (
	"context"

	"github.com/cryptopay-dev/narada/clients"
	"github.com/cryptopay-dev/narada/lock"
	"github.com/cryptopay-dev/narada/worker"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Narada struct {
		providers []interface{}
		logger    *logrus.Logger
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
	t.logger.Fatal(err)
}

func New(opts Options, providers ...interface{}) *Narada {
	config, err := NewConfig(opts.EnvPrefix)
	if err != nil {
		logger, _ := NewLogger(viper.New())
		logger.WithField("error", err).Fatal("error reading configuration")
	}

	config.SetDefault("app.name", opts.Name)
	config.SetDefault("app.version", opts.Version)

	logger, err := NewLogger(config)
	if err != nil {
		logger, _ := NewLogger(viper.New())
		logger.WithField("error", err).Fatal("error creating logger from configuration")
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
		t.logger.Fatalf("error stopping: %v", err)
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
		// Setting default logger to discard
		fx.Logger(NewNopLogger()),

		fx.ErrorHook(t),

		fx.Provide(
			// Fundamentals
			func() *logrus.Logger { return t.logger },
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
