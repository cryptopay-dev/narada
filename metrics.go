package tuktuk

import "github.com/prometheus/client_golang/prometheus/promhttp"

func NewMetricsInvoke(ms *Multiserver) error {
	return ms.Add("metrics", promhttp.Handler())
}
