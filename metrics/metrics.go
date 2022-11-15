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
	CurrentHeartbeatConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locate_current_heartbeat_connections",
			Help: "Number of currently active Heartbeat connections.",
		},
		[]string{"experiment"},
	)

	// PrometheusHealthCollectionDuration is a histogram that tracks the latency of the
	// handler that collects Prometheus health signals.
	PrometheusHealthCollectionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "prometheus_health_collection_duration",
			Help: "A histogram of request latencies to the Prometheus health signal handler.",
		},
		[]string{"code"},
	)

	// HeartbeatRegistrationsTotal counts the total number of registration
	HeartbeatRegistrationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heartbeat_registrations_total",
			Help: "Number of heartbeat registration requests",
		},
		[]string{"experiment", "status"},
	)

	// PortChecksTotal counts the number of port checks performed by the Heartbeat
	// Service.
	PortChecksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heartbeat_port_checks_total",
			Help: "Number of port checks the HBS has done",
		},
		[]string{"status"},
	)

	// KubernetesRequestsTotal counts the number of requests from the Heartbeat
	// Service to the Kubernetes API server.
	KubernetesRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heartbeat_kubernetes_requests_total",
			Help: "Number of requests from the HBS to the Kubernetes API",
		},
		[]string{"status"},
	)

	// HealthEndpointChecksTotal counts the number of local /health endpoint
	// checks performed by the Heartbeat Service.
	HealthEndpointChecksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heartbeat_health_endpoint_checks_total",
			Help: "Number of local /health endpoint checks the HBS has done",
		},
		[]string{"status"},
	)

	// KubernetesRequestTimeHistogram tracks the request latency from the Heartbeat
	// Service to the Kubernetes API server (in seconds).
	KubernetesRequestTimeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "heartbeat_kubernetes_request_time_histogram",
			Help: "Request time from the HBS to the Kubernetes API server (seconds)",
		},
		[]string{"healthy"},
	)
)
