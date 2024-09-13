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
		servers map[string]*http.Server
		logger  *logrus.Logger
		config  *viper.Viper
	}

	server struct {
		name    string
		handler http.Handler
		log     logrus.FieldLogger
	}

	serverOption func(*server)

	Healthchecker func() error
)

var noopHealthcheck = func() error { return nil }

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

func (ms *Multiserver) Add(name string, handler http.Handler, opts ...serverOption) error {
	s := &server{name: name, handler: handler, log: ms.logger.WithField("server", name)}
	for _, o := range opts {
		o(s)
	}

	key := fmt.Sprintf("bind.%s", s.name)
	addr := ms.config.GetString(key)
	if addr == "" {
		return fmt.Errorf("error starting server %s, empty address in config [%s]", s.name, key)
	}

	if _, ok := ms.servers[s.name]; ok {
		return fmt.Errorf("error adding server, duplicate key: %s", s.name)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: s.handler,
	}

	ms.servers[s.name] = srv

	return nil
}

func (ms *Multiserver) AddHealthcheck(name, path string, check Healthchecker) error {
	log := ms.logger.WithField("server", name)

	mux := http.NewServeMux()
	mux.HandleFunc(path, newHealthcheckHandler(log, check))
	mux.HandleFunc("/", newNotFoundHealthcheckHandler(log))

	return ms.Add(name, withServerHealthcheckSummary(name, mux))
}

func WithHealthcheck(path string) serverOption {
	return func(s *server) {
		mux := http.NewServeMux()
		mux.HandleFunc(path, newHealthcheckHandler(s.log, noopHealthcheck))
		mux.Handle("/", s.handler)

		s.handler = withServerHealthcheckSummary(s.name, mux)
	}
}

func newHealthcheckHandler(log logrus.FieldLogger, check Healthchecker) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		log = log.WithField("path", r.URL.Path)

		if err := check(); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			log.WithError(err).Error("healthcheck failed")
			return
		}

		rw.WriteHeader(http.StatusOK)
		log.Debug("healthcheck served")
	}
}

func newNotFoundHealthcheckHandler(log logrus.FieldLogger) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		log.WithField("path", r.URL.Path).Error("unknown healthcheck request")
	}
}
