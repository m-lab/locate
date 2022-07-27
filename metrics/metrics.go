package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts the number of 'nearest' requests served by
	// the Locate service.
	//
	// Example usage:
	// metrics.RequestsTotal.WithLabelValues("200").Inc()
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_requests_total",
			Help: "Number of 'nearest' requests served by the Locate service.",
		},
		[]string{"status"},
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

	// HeartbeatConnectionsTotal counts the number of active Heartbeat
	// connections.
	//
	// Example usage:
	// metrics.HeartbeatConnectionsTotal.Inc()
	HeartbeatConnectionsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "locate_heartbeat_connections_total",
			Help: "Number of active Heartbeat connections.",
		},
	)
)
