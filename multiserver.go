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
		config  *viper.Viper
	}
)

func NewMultiServerLauncher(ms *Multiserver, logger *logrus.Logger) {
	logger.Info("starting HTTP server if we need to")
}

func (ms *Multiserver) Add(name string, handler http.Handler) error {
	key := fmt.Sprintf("bind.%s", name)
	addr := ms.config.GetString(key)
	if addr == "" {
		return fmt.Errorf("error starting server %s, empty address in config [%s]", name, key)
	}

	ms.servers[name] = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return nil
}

func NewMultiServers(config *viper.Viper, logger *logrus.Logger, lc fx.Lifecycle) (*Multiserver, error) {
	servers := make(map[string]*http.Server)

	// Default bindings for metrics & pprof
	config.SetDefault("bind.pprof", ":9001")
	config.SetDefault("bind.metrics", ":9002")

	ms := &Multiserver{
		servers: servers,
		logger:  logger,
		config:  config,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			for name, s := range ms.servers {
				ms.logger.WithFields(logrus.Fields{
					"server_name": name,
					"address":     s.Addr,
				}).Info("starting server")
				go func(name string, s *http.Server) {
					if err := s.ListenAndServe(); err != nil {
						if err == http.ErrServerClosed {
							return
						}

						ms.logger.WithFields(logrus.Fields{
							"server_name": name,
							"error":       err,
						}).Error("error starting server")
					}
				}(name, s)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			for name, s := range ms.servers {
				ms.logger.WithField("server_name", name).Info("shutdown server")

				if err := s.Shutdown(ctx); err != nil {
					ms.logger.WithFields(logrus.Fields{
						"server_name": name,
						"error":       err,
					}).Error("error while trying to shutdown server")
				}
			}

			return nil
		},
	})

	return ms, nil
}
