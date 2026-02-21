package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// HTTPRequestDuration tracks HTTP request latency.
var HTTPRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "nightowl",
		Subsystem: "api",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	},
	[]string{"method", "path", "status"},
)

// AlertsDeduplicatedTotal counts the number of deduplicated alerts.
var AlertsDeduplicatedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "deduplicated_total",
		Help:      "Total number of deduplicated alerts.",
	},
)

// NewMetricsRegistry creates a Prometheus registry with default and custom collectors.
func NewMetricsRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		HTTPRequestDuration,
		AlertsDeduplicatedTotal,
	)
	return reg
}
