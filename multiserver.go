package narada

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
		servers map[string]*server
		logger  *logrus.Logger
		config  *viper.Viper
	}
)

func (ms *Multiserver) Add(name string, handler http.Handler) error {
	key := fmt.Sprintf("bind.%s", name)
	addr := ms.config.GetString(key)
	if addr == "" {
		return fmt.Errorf("error starting server %s, empty address in config [%s]", name, key)
	}

	if _, ok := ms.servers[name]; ok {
		return fmt.Errorf("error adding server, duplicate key: %s", name)
	}

	srv := &server{
		s: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}

	if ms.config.IsSet("tls." + name) {
		srv.tls = &tlsConfig{
			certFile: ms.config.GetString("tls." + name + ".cert_file"),
			keyFile:  ms.config.GetString("tls." + name + ".key_file"),
		}
	}

	ms.servers[name] = srv

	return nil
}

func NewMultiServers(config *viper.Viper, logger *logrus.Logger, lc fx.Lifecycle) (*Multiserver, error) {
	servers := make(map[string]*server)

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
					"address":     s.s.Addr,
				}).Info("starting server")
				go func(name string, s *server) {
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

type (
	server struct {
		s   *http.Server
		tls *tlsConfig
	}

	tlsConfig struct {
		certFile string
		keyFile  string
	}
)

func (s *server) ListenAndServe() error {
	if s.tls != nil {
		return s.s.ListenAndServeTLS(s.tls.certFile, s.tls.keyFile)
	}

	return s.s.ListenAndServe()
}

func (s *server) Shutdown(ctx context.Context) error {
	return s.s.Shutdown(ctx)
}
