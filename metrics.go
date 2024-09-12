package narada

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	serverHealthcheckSummary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Subsystem:  "server",
			Name:       "healthcheck_duration_seconds",
			Help:       "time elapsed for healthcheck response",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"server", "code", "method"},
	)
)

func withServerHealthcheckSummary(server string, handler http.Handler) http.Handler {
	return promhttp.InstrumentHandlerDuration(
		serverHealthcheckSummary.MustCurryWith(prometheus.Labels{"server": server}),
		handler,
	)
}

func NewMetricsInvoke(ms *Multiserver) error {
	return ms.Add("metrics", promhttp.Handler())
}
