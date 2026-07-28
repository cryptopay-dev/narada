package narada

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

type (
	Multiserver struct {
		servers map[string]*http.Server
		logger  *slog.Logger
		config  *viper.Viper
	}

	server struct {
		name    string
		handler http.Handler
		log     *slog.Logger
	}

	serverOption func(*server)

	Healthchecker func() error
)

var noopHealthcheck = func() error { return nil }

func NewMultiServers(config *viper.Viper, logger *slog.Logger, lc fx.Lifecycle) (*Multiserver, error) {
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
				ms.logger.Info("starting server",
					"server_name", name,
					"address", s.Addr,
				)
				go func(name string, s *http.Server) {
					if err := s.ListenAndServe(); err != nil {
						if err == http.ErrServerClosed {
							return
						}

						ms.logger.Error("error starting server",
							"server_name", name,
							Err(err),
						)
					}
				}(name, s)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			for name, s := range ms.servers {
				ms.logger.Info("shutdown server", "server_name", name)

				if err := s.Shutdown(ctx); err != nil {
					ms.logger.Error("error while trying to shutdown server",
						"server_name", name,
						Err(err),
					)
				}
			}

			return nil
		},
	})

	return ms, nil
}

func (ms *Multiserver) Add(name string, handler http.Handler, opts ...serverOption) error {
	s := &server{name: name, handler: handler, log: ms.logger.With("server", name)}
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
	log := ms.logger.With("server", name)

	mux := http.NewServeMux()
	mux.HandleFunc(path, newHealthcheckHandler(log, check))
	mux.HandleFunc("/", newNotFoundHealthcheckHandler(log))

	return ms.Add(name, withServerHealthcheckSummary(name, mux))
}

func WithHealthcheck(path string) serverOption {
	return func(s *server) {
		hc := newHealthcheckHandler(s.log, noopHealthcheck)

		mux := http.NewServeMux()
		mux.Handle(path, withServerHealthcheckSummary(s.name, hc))
		mux.Handle("/", s.handler)

		s.handler = mux
	}
}

func newHealthcheckHandler(log *slog.Logger, check Healthchecker) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		log := log.With("path", r.URL.Path)

		if err := check(); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			log.Error("healthcheck failed", Err(err))
			return
		}

		rw.WriteHeader(http.StatusOK)
		log.Debug("healthcheck served")
	}
}

func newNotFoundHealthcheckHandler(log *slog.Logger) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		log.Error("unknown healthcheck request", "path", r.URL.Path)
	}
}
