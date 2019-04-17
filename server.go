package tuktuk

import (
	"net/http"

	"go.uber.org/fx"
)

type (
	ServerResult struct {
		fx.Out

		Server Server `group:"servers"`
	}

	Server struct {
		Name    string
		Handler http.Handler
	}
)

func NewServer(name string, handler http.Handler) ServerResult {
	return ServerResult{
		Server: Server{
			Name:    name,
			Handler: handler,
		},
	}
}
