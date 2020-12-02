package worker

import "github.com/prometheus/client_golang/prometheus"

var (
	objectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

	workerSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Subsystem:  "workers",
			Name:       "job_duration_seconds",
			Help:       "time elapsed to doing single worker job",
			Objectives: objectives,
		},
		[]string{"name"},
	)
)

func init() {
	prometheus.MustRegister(workerSummary)
}
