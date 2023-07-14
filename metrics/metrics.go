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
		[]string{"type", "condition", "status"},
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

	// LocateHealthStatus exposes the health status collected by the Locate Service.
	LocateHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "locate_health_status",
			Help: "Health status collected by the Locate Service.",
		},
		[]string{"experiment"},
	)

	// ImportMemorystoreTotal counts the number of times the Locate Service has imported
	// the data in Memorystore.
	ImportMemorystoreTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_import_memorystore_total",
			Help: "Number of times the Locate Service has imported the data in Memorystore.",
		},
		[]string{"status"},
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

	// ServerDistanceRanking is a histogram that tracks the ranked distance of the returned servers
	// with respect to the client.
	// Numbering is zero-based.
	//
	// Example usage (the 2nd closest server to the client is returned as the 1st server in the list):
	// metrics.ServerDistanceRanking.WithLabelValues(0).Observe(1)
	ServerDistanceRanking = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "locate_server_distance_ranking",
			Help:    "A histogram of server selection rankings with respect to distance from the client.",
			Buckets: prometheus.LinearBuckets(0, 1, 20),
		},
		[]string{"index"},
	)

	// MetroDistanceRanking is a histogram that tracks the ranked distance of the returned metros
	// with respect to the client.
	// Numbering is zero-based.
	//
	// Example usage (the 1st server in the list is in the 2nd metro closest to the client):
	// metrics.MetroDistanceRanking.WithLabelValues(0).Observe(1)
	MetroDistanceRanking = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "locate_metro_distance_ranking",
			Help:    "A histogram of metro selection rankings with respect to distance from the client.",
			Buckets: prometheus.LinearBuckets(0, 1, 20),
		},
		[]string{"index"},
	)

	// ConnectionRequestsTotal counts the number of (re)connection requests the Heartbeat Service
	// makes to the Locate Service.
	ConnectionRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "connection_requests_total",
			Help: "Number of connection requests from the HBS to the Locate Service.",
		},
		[]string{"status"},
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

	// RegistrationUpdateTime tracks the time when a new registration message
	// is retrieved from siteinfo.
	RegistrationUpdateTime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "heartbeat_registration_update_time",
			Help: "Time of new registration retrieval from siteinfo.",
		},
	)
)
