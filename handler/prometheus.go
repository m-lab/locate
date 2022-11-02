package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var (
	timeout         = static.PrometheusCheckPeriod
	errCouldNotCast = errors.New("could not cast metric to vector")

	// End-to-end query parameters.
	e2eQuery = "script_success"
	e2eLabel = model.LabelName("fqdn")
	// The script was successful if the value != 0.
	e2eFunction = func(v float64) bool {
		return v != 0
	}

	// GMX query parameters.
	gmxQuery = "gmx_machine_maintenance"
	gmxLabel = model.LabelName("machine")
	// The machine is not in maintenance if the value = 0.
	gmxFunction = func(v float64) bool {
		return v == 0
	}
)

// Prometheus is a handler that collects Prometheus health signals.
func (c *Client) Prometheus(rw http.ResponseWriter, req *http.Request) {
	hostnames, err := c.query(req.Context(), e2eQuery, e2eLabel, e2eFunction)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	machines, err := c.query(req.Context(), gmxQuery, gmxLabel, gmxFunction)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	c.UpdatePrometheus(hostnames, machines)
	rw.WriteHeader(http.StatusOK)
}

// query performs the provided PromQL query.
func (c *Client) query(ctx context.Context, query string, labelName model.LabelName, f func(v float64) bool) (map[string]bool, error) {
	result, _, err := c.PrometheusClient.Query(ctx, query, time.Now(), prom.WithTimeout(timeout))
	if err != nil {
		return nil, err
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, errCouldNotCast
	}

	return getMetrics(vector, labelName, f), nil
}

// getMetrics returns a map of labels to bool values from a Vector, based on the function parameter.
func getMetrics(vector model.Vector, labelName model.LabelName, f func(v float64) bool) map[string]bool {
	metrics := map[string]bool{}

	for _, elem := range vector {
		label := string(elem.Metric[labelName])
		value := float64(elem.Value)
		metrics[label] = f(value)
	}

	return metrics
}
