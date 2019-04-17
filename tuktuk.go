package tuktuk

import (
	"github.com/m1ome/tuktuk/clients"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Tuktuk struct {
		providers []interface{}
		logger    *logrus.Logger
		config    *viper.Viper
	}
)

func (t Tuktuk) HandleError(err error) {
	t.logger.Fatal(err)
}

func New(providers ...interface{}) *Tuktuk {
	config, err := NewConfig()
	if err != nil {
		logger, _ := NewLogger(viper.New())
		logger.WithField("error", err).Fatal("error reading configuration")
	}

	logger, err := NewLogger(config)
	if err != nil {
		logger, _ := NewLogger(viper.New())
		logger.WithField("error", err).Fatal("error creating logger from configuration")
	}

	return &Tuktuk{
		providers: providers,
		logger:    logger,
		config:    config,
	}
}

func (t *Tuktuk) Start(fn interface{}) {
	// Creating application
	app := fx.New(
		// Setting default logger to discard
		fx.Logger(NewNopLogger()),

		fx.ErrorHook(t),

		fx.Provide(
			// Fundamentals
			NewConfig,
			NewSentry,
			NewLogger,

			// Default servers pprof & prometheus metrics
			NewProfiler,
			NewMetrics,

			// Servers handling
			NewMultiServers,

			// Workers handling
			NewWorkers,

			// Clients
			clients.NewPostgreSQL,
		),

		fx.Provide(t.providers...),

		fx.Invoke(NewMultiServerLauncher, NewWorkerLauncher, fn),
	)

	app.Run()
}

func (t *Tuktuk) Run() {
	t.Start(func(logger *logrus.Logger) {
		logger.Info("starting application")
	})
}
