package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/m-lab/go/host"
	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var (
	timeout         = static.PrometheusCheckPeriod
	errCouldNotCast = errors.New("could not cast metric to vector")

	// End-to-end query parameters.
	// The raw query should be unfiltered.
	e2eQuery = "script_success"
	e2eLabel = model.LabelName("fqdn")
	// The script was successful if the value != 0.
	e2eFunction = func(v float64) bool {
		return v != 0
	}

	// GMX query parameters.
	gmxQuery = "gmx_machine_maintenance"
	// The raw query should be unfiltered.
	gmxLabel = model.LabelName("machine")
	// The machine is not in maintenance if the value = 0.
	gmxFunction = func(v float64) bool {
		return v == 0
	}
)

// Prometheus is a handler that collects Prometheus health signals.
func (c *Client) Prometheus(rw http.ResponseWriter, req *http.Request) {
	err := c.updatePrometheus(req.Context(), "")
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}

// UpdatePrometheusForMachine updates the Prometheus signals for a single machine hostname.
func (c *Client) UpdatePrometheusForMachine(ctx context.Context, hostname string) error {
	name, err := host.Parse(hostname)
	if err != nil {
		log.Printf("Error parsing hostname %s", hostname)
		return err
	}

	machine := name.String()
	err = c.updatePrometheus(ctx, fmt.Sprintf("machine=%q", machine))
	if err != nil {
		log.Printf("Error updating Prometheus signals for machine %s", machine)
	}
	return err
}

func (c *Client) updatePrometheus(ctx context.Context, filter string) error {
	hostnames, err := c.query(ctx, e2eQuery, filter, e2eLabel, e2eFunction)
	if err != nil {
		log.Printf("Error querying Prometheus for %s metric: %v", e2eQuery, err)
		return err
	}

	machines, err := c.query(ctx, gmxQuery, filter, gmxLabel, gmxFunction)
	if err != nil {
		log.Printf("Error querying Prometheus for %s metric: %v", gmxQuery, err)
		return err
	}

	err = c.UpdatePrometheus(hostnames, machines)
	if err != nil {
		log.Printf("Error updating internal Prometheus state: %v", err)
		return err
	}

	return nil
}

// query performs the provided PromQL query.
func (c *Client) query(ctx context.Context, query, filter string, labelName model.LabelName, f func(v float64) bool) (map[string]bool, error) {
	result, _, err := c.PrometheusClient.Query(ctx, formatQuery(query, filter), time.Now(), prom.WithTimeout(timeout))
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

func formatQuery(query, filter string) string {
	if filter != "" {
		return fmt.Sprintf("%s{%s}", query, filter)
	}
	return query
}
