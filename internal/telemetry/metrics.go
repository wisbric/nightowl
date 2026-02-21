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

// AlertsAgentResolvedTotal counts alerts auto-resolved by agents.
var AlertsAgentResolvedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "agent_resolved_total",
		Help:      "Total number of alerts auto-resolved by agents.",
	},
)

// AlertsReceivedTotal counts received alerts by source and severity.
var AlertsReceivedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "received_total",
		Help:      "Total number of alerts received.",
	},
	[]string{"source", "severity"},
)

// AlertProcessingDuration tracks webhook alert processing latency by source.
var AlertProcessingDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "nightowl",
		Subsystem: "alert",
		Name:      "processing_duration_seconds",
		Help:      "Alert webhook processing duration in seconds.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	},
	[]string{"source"},
)

// KBHitsTotal counts successful knowledge base enrichment matches.
var KBHitsTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Name:      "kb_hits_total",
		Help:      "Total number of successful knowledge base enrichment matches.",
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
		AlertsAgentResolvedTotal,
		AlertsReceivedTotal,
		AlertProcessingDuration,
		KBHitsTotal,
	)
	return reg
}
