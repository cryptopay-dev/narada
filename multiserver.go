package tuktuk

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Multiserver struct {
		servers map[string]*http.Server
		logger  *logrus.Logger
	}

	MultiserverParams struct {
		fx.In

		Config  *viper.Viper
		Logger  *logrus.Logger
		Lc      fx.Lifecycle
		Servers []Server `group:"servers"`
	}
)

func NewMultiServerLauncher(ms *Multiserver, logger *logrus.Logger) {
	logger.Info("starting HTTP server if we need to")
}

func NewMultiServers(opts MultiserverParams) (*Multiserver, error) {
	servers := make(map[string]*http.Server)

	// Default bindings for metrics & pprof
	opts.Config.SetDefault("bind.pprof", ":9001")
	opts.Config.SetDefault("bind.metrics", ":9002")

	for _, server := range opts.Servers {
		// Getting server address
		key := fmt.Sprintf("bind.%s", server.Name)
		addr := opts.Config.GetString(key)
		if addr == "" {
			return nil, fmt.Errorf("error starting server %s, empty address in config [%s]", server.Name, key)
		}

		servers[server.Name] = &http.Server{
			Addr:    addr,
			Handler: server.Handler,
		}
	}

	opts.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for name, server := range servers {
				opts.Logger.WithFields(logrus.Fields{
					"server_name": name,
					"address":     server.Addr,
				}).Info("starting server")
				go func(name string, s *http.Server) {
					if err := s.ListenAndServe(); err != nil {
						if err == http.ErrServerClosed {
							return
						}

						opts.Logger.WithFields(logrus.Fields{
							"server_name": name,
							"error":       err,
						}).Error("error starting server")
					}
				}(name, server)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			for name, server := range servers {
				opts.Logger.WithField("server_name", name).Info("shutdown server")

				if err := server.Shutdown(ctx); err != nil {
					opts.Logger.WithFields(logrus.Fields{
						"server_name": name,
						"error":       err,
					}).Error("error while trying to shutdown server")
				}
			}

			return nil
		},
	})

	return &Multiserver{
		servers: servers,
		logger:  opts.Logger,
	}, nil
}
