package telemetry

import "github.com/prometheus/client_golang/prometheus"

var AlertsDeduplicatedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "deduplicated_total",
		Help:      "Total number of deduplicated alerts.",
	},
)

var AlertsAgentResolvedTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "agent_resolved_total",
		Help:      "Total number of alerts auto-resolved by agents.",
	},
)

var AlertsReceivedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "received_total",
		Help:      "Total number of alerts received.",
	},
	[]string{"source", "severity"},
)

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

var KBHitsTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Name:      "kb_hits_total",
		Help:      "Total number of successful knowledge base enrichment matches.",
	},
)

var SlackNotificationsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "slack",
		Name:      "notifications_total",
		Help:      "Total number of Slack notifications sent by type.",
	},
	[]string{"type"},
)

var AlertsEscalatedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "nightowl",
		Subsystem: "alerts",
		Name:      "escalated_total",
		Help:      "Total number of alerts escalated by tier.",
	},
	[]string{"tier"},
)

// All returns all NightOwl-specific metrics for registration.
func All() []prometheus.Collector {
	return []prometheus.Collector{
		AlertsDeduplicatedTotal,
		AlertsAgentResolvedTotal,
		AlertsReceivedTotal,
		AlertProcessingDuration,
		KBHitsTotal,
		SlackNotificationsTotal,
		AlertsEscalatedTotal,
	}
}
