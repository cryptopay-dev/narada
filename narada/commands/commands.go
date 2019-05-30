package commands

import (
	"context"

	"github.com/m1ome/narada"
	"github.com/m1ome/narada/clients"

	"github.com/sirupsen/logrus"
	"go.uber.org/fx"
)

type consoleErrorHandler struct {
	logger *logrus.Logger
}

func (c consoleErrorHandler) HandleError(err error) {
	c.logger.Fatal(err)
}

func Run(ctx context.Context, fn interface{}) error {
	h := consoleErrorHandler{logrus.New()}
	// Creating application
	app := fx.New(
		fx.Logger(narada.NewNopLogger()),
		fx.ErrorHook(h),

		fx.Provide(
			// Fundamentals
			narada.NewConfig,
			narada.NewLogger,

			// Clients
			clients.NewPostgreSQL,
			clients.NewRedis,
		),

		fx.Invoke(fn),
	)

	// Running application
	return app.Start(ctx)
}
