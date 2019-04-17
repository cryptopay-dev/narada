package tuktuk

import "github.com/prometheus/client_golang/prometheus/promhttp"

func NewMetrics() ServerResult {
	return NewServer("metrics", promhttp.Handler())
}
