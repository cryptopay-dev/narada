package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/labstack/echo"
	"github.com/m1ome/tuktuk"
	"github.com/sirupsen/logrus"
)

var (
	jobsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "jobs_processed",
		Help: "Total number of jobs processed",
	})
)

func NewApiServer() tuktuk.ServerResult {
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

	return tuktuk.NewServer("api", e)
}

func NewWorkers(logger *logrus.Logger) tuktuk.JobResult {
	times := 0

	return tuktuk.NewJob(tuktuk.Job{
		Name:   "dummy",
		Period: time.Second,
		Handler: func() {
			times++

			jobsProcessed.Inc()
			logger.Infof("job run %d times", times)
		},
		Immediately: true,
	})
}

func main() {
	tuktuk.New(
		NewApiServer,
		NewWorkers,
	).Run()
}
