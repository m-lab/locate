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

	// RateLimitedTotal tracks the number of rate-limited requests by client name.
	RateLimitedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_rate_limited_total",
			Help: "Total number of rate-limited requests by client name",
		},
		[]string{"clientname"},
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

	// LocateMemorystoreRequestDuration is a histogram that tracks the latency of
	// requests from the Locate to Memorystore.
	LocateMemorystoreRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "locate_memorystore_request_duration",
			Help: "A histogram of request latency to Memorystore.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1,
				2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30},
		},
		[]string{"type", "field", "status"},
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

	// RequestHandlerDuration is a histogram that tracks the latency of each request handler.
	RequestHandlerDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "locate_request_handler_duration",
			Help: "A histogram of latencies for each request handler.",
		},
		[]string{"path", "code"},
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
		[]string{"type", "status"},
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

	// HealthTransmissionDuration is a histogram for the latency of the heartbeat
	// to assess local health and send it to the Locate.
	HealthTransmissionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "heartbeat_health_transmission_duration",
			Help:    "Latency for the heartbeat to assess local health and send it.",
			Buckets: prometheus.LinearBuckets(0, 2, 16),
		},
		[]string{"score"},
	)
)
