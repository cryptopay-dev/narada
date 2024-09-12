package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/cryptopay-dev/narada"
	"github.com/cryptopay-dev/narada/worker"
	"github.com/sirupsen/logrus"
)

func Run(ms *narada.Multiserver, workers *worker.Workers, logger *logrus.Logger) error {
	// Atomic counter
	var counter uint64

	// Adding servers
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)

		greetings := fmt.Sprintf("Hello, user! Counter is %d", atomic.LoadUint64(&counter))
		if _, err := writer.Write([]byte(greetings)); err != nil {
			logger.WithError(err).Error("error writing response")
		}
	})
	if err := ms.Add("api", mux, narada.WithHealthcheck("/health")); err != nil {
		return err
	}

	if err := ms.AddHealthcheck("health", "/health", func() error {
		if rand.Intn(2) == 1 {
			return errors.New("healtcheck failed")
		}
		return nil
	}); err != nil {
		return err
	}

	// Adding workers
	job := worker.Job{
		Name: "counter",
		Handler: func(ctx context.Context) {
			atomic.AddUint64(&counter, 1)
		},
		Period:      10 * time.Second,
		Immediately: true,
	}
	workers.Add(job)

	return nil
}

func main() {
	narada.New(narada.Options{
		Name:    "api_server",
		Version: "development",
	}).Start(Run)
}
