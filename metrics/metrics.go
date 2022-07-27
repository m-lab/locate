package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ConnectionClosedTotal counts the number of times connections
	// are closed.
	//
	// Example usage:
	// metrics.ConnectionClosedTotal.Inc()
	ConnectionClosedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "locate_connection_closed_total",
			Help: "Number of connections that have been closed.",
		},
	)

	// ReconnectionsTotal counts the number of times a reconnection
	// attempt is made.
	//
	// Example usage:
	// metrics.ReconnectionsTotal.WithLabelValues("success").Inc()
	ReconnectionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "locate_reconnections_total",
			Help: "Number of reconnection attempts.",
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
)
