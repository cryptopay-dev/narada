package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/m1ome/tuktuk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

var (
	jobsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_processed",
		Help: "Total number of jobs processed",
	})
)

func App(ms *tuktuk.Multiserver, worker *tuktuk.Workers, logger *logrus.Logger) error {
	// Creating server
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true

	e.GET("/", func(ctx echo.Context) error {
		name := ctx.Param("name")
		if name == "" {
			name = "stranger"
		}

		return ctx.String(http.StatusOK, fmt.Sprintf("Hello, %s!", name))
	})

	// Adding jobs
	times := 0
	worker.Add(tuktuk.Job{
		Name:   "dummy",
		Period: time.Second,
		Handler: func() {
			times++

			jobsProcessed.Inc()
			logger.Infof("job run %d times", times)
		},
		Immediately: true,
	})

	return ms.Add("api", e)
}

func main() {
	tuktuk.New().Start(App)
}
