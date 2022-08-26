package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts the number of requests served by
	// the Locate service.
	//
	// Example usage:
	// metrics.RequestsTotal.WithLabelValues("nearest", "200").Inc()
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_requests_total",
			Help: "Number of requests served by the Locate service.",
		},
		[]string{"type", "status"},
	)

	// AppEngineTotal counts the number of times App Engine headers are
	// used to try to find the client location.
	//
	// Example usage:
	// metrics.AppEngineTotal.WithLabelValues("US").Inc()
	AppEngineTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_app_engine_total",
			Help: "Number of times App Engine is used to find the client location.",
		},
		[]string{"country"},
	)

	// CurrentHeartbeatConnections counts the number of currently active
	// Heartbeat connections.
	//
	// Example usage:
	// metrics.CurrentHeartbeatConnections.Inc()
	CurrentHeartbeatConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "locate_current_heartbeat_connections",
			Help: "Number of currently active Heartbeat connections.",
		},
	)

	// KubernetesRequestTimeHistogram tracks the request latency from the Heartbeat
	// Service to the Kubernetes API server (in seconds).
	KubernetesRequestTimeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heartbeat_kubernetes_request_time_histogram",
			Help:    "Request time from the HBS to the Kubernetes API server (seconds)",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		[]string{"healthy"},
	)
)
