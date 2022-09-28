package health

import (
	"net/http"

	"github.com/m-lab/locate/metrics"
)

var (
	healthAddress = "http://localhost:8000/health"
)

// checkHealthEndpoint makes a call to the the local /health endpoint.
// It returns an error if the HTTP request was not successful.
// It returns true only if the returned HTTP status code equals 200 (OK).
func checkHealthEndpoint() (bool, error) {
	resp, err := http.Get(healthAddress)
	if err != nil {
		metrics.HealthEndpointChecksTotal.WithLabelValues("HTTP request error").Inc()
		return false, err
	}

	if resp.StatusCode == http.StatusOK {
		metrics.HealthEndpointChecksTotal.WithLabelValues("OK").Inc()
		return true, nil
	}

	metrics.HealthEndpointChecksTotal.WithLabelValues(http.StatusText(resp.StatusCode)).Inc()
	return false, nil
}
