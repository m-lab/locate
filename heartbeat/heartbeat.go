package heartbeat

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
)

var (
	errInvalidArgument = errors.New("argument is invalid")
	errPrometheus      = errors.New("error saving Prometheus entry")
)

type heartbeatStatusTracker struct {
	MemorystoreClient[v2.HeartbeatMessage]
	instances map[string]v2.HeartbeatMessage
	mu        sync.RWMutex
	stop      chan bool
}

// MemorystoreClient is a client for reading and writing data in Memorystore.
// The interface takes in a type argument which specifies the types of values
// that are stored and can be retrived.
type MemorystoreClient[V any] interface {
	Put(key string, field string, value redis.Scanner, expire bool) error
	GetAll() (map[string]V, error)
}

// NewHeartbeatStatusTracker returns a new StatusTracker implementation that uses
// a Memorystore client to cache (and later import) instance data from the Heartbeat Service.
// StopImport() must be called to release resources.
func NewHeartbeatStatusTracker(client MemorystoreClient[v2.HeartbeatMessage]) *heartbeatStatusTracker {
	h := &heartbeatStatusTracker{
		MemorystoreClient: client,
		instances:         make(map[string]v2.HeartbeatMessage),
		stop:              make(chan bool),
	}

	// Start import loop.
	go func(h *heartbeatStatusTracker) {
		ticker := *time.NewTicker(static.MemorystoreExportPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-h.stop:
				return
			case <-ticker.C:
				h.importMemorystore()
			}
		}
	}(h)

	return h
}

// RegisterInstance adds a new v2.Registration message to the Memorystore client and keeps it
// locally.
func (h *heartbeatStatusTracker) RegisterInstance(rm v2.Registration) error {
	hostname := rm.Hostname
	if err := h.Put(hostname, "Registration", &rm, true); err != nil {
		return err
	}

	h.registerInstance(hostname, rm)
	return nil
}

// UpdateHealth updates the v2.Health field for the instance in the Memorystore client and
// updates it locally.
func (h *heartbeatStatusTracker) UpdateHealth(hostname string, hm v2.Health) error {
	if err := h.Put(hostname, "Health", &hm, true); err != nil {
		return err
	}
	return h.updateHealth(hostname, hm)
}

// UpdatePrometheus updates the v2.Prometheus field for the instances.
func (h *heartbeatStatusTracker) UpdatePrometheus(hostnames, machines map[string]bool) error {
	var err error
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, instance := range h.instances {
		pm := constructPrometheusMessage(instance, hostnames, machines)
		if pm != nil {
			updateErr := h.updatePrometheusMessage(instance, pm)

			if updateErr != nil {
				err = errPrometheus
			}
		}
	}

	return err
}

// Instances returns a mapping of all the v2.HeartbeatMessage instance keys to
// their values.
func (h *heartbeatStatusTracker) Instances() map[string]v2.HeartbeatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c := make(map[string]v2.HeartbeatMessage, len(h.instances))
	for k, v := range h.instances {
		c[k] = v
	}

	return c
}

// StopImport stops importing instance data from the Memorystore.
// It must be called to release resources.
func (h *heartbeatStatusTracker) StopImport() {
	h.stop <- true
}

func (h *heartbeatStatusTracker) registerInstance(hostname string, rm v2.Registration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.instances[hostname] = v2.HeartbeatMessage{Registration: &rm}
}

func (h *heartbeatStatusTracker) updateHealth(hostname string, hm v2.Health) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if instance, found := h.instances[hostname]; found {
		instance.Health = &hm
		h.instances[hostname] = instance
		return nil
	}

	return fmt.Errorf("failed to find %s instance for health update", hostname)
}

// updatePrometheusMessage updates the v2.Prometheus field for a specific instance
// in Memorystore and locally.
func (h *heartbeatStatusTracker) updatePrometheusMessage(instance v2.HeartbeatMessage, pm *v2.Prometheus) error {
	hostname := instance.Registration.Hostname

	// Update in Memorystore.
	err := h.Put(hostname, "Prometheus", pm, false)
	if err != nil {
		return err
	}

	// Update locally.
	instance.Prometheus = pm
	h.instances[hostname] = instance
	return nil
}

func (h *heartbeatStatusTracker) importMemorystore() {
	values, err := h.GetAll()

	if err == nil {
		h.instances = values
		h.updateMetrics()
	}
}

// updateMetrics updates a Prometheus Gauge with the number of healthy instances per
// experiment.
// Note that if an experiment is deleted (i.e., there are no more experiment instances),
// the metric will still report the last known count.
func (h *heartbeatStatusTracker) updateMetrics() {
	healthy := h.getHealthy()

	for experiment, count := range healthy {
		metrics.LocateHealthStatus.WithLabelValues(experiment).Set(count)
	}
}

// getHealthy returns a map of experiments to their corresponding count of healthy
// instances.
func (h *heartbeatStatusTracker) getHealthy() map[string]float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	healthy := make(map[string]float64)
	for _, instance := range h.instances {
		if isHealthy(instance) {
			healthy[instance.Registration.Experiment]++
		}
	}

	return healthy
}

// constructPrometheusMessage constructs a v2.Prometheus message for a specific instance
// from a map of hostname/machine Prometheus data.
// If no information is available for the instance, it returns nil.
func constructPrometheusMessage(instance v2.HeartbeatMessage, hostnames, machines map[string]bool) *v2.Prometheus {
	if instance.Registration == nil {
		return nil
	}

	var hostHealthy, hostFound, machineHealthy, machineFound bool

	// Get Prometheus health data for the service hostname.
	hostname := instance.Registration.Hostname
	hostHealthy, hostFound = hostnames[hostname]

	// Get Prometheus health data for the machine.
	parts, err := host.Parse(hostname)
	if err == nil {
		machineHealthy, machineFound = machines[parts.String()]
	}

	// Create Prometheus health message.
	if hostFound || machineFound {
		// If Prometheus did not return any data about one of host or machine,
		// treat it as healthy.
		health := (!hostFound || hostHealthy) && (!machineFound || machineHealthy)
		return &v2.Prometheus{Health: health}
	}

	// If no Prometheus data is available for either the host or machine (both missing),
	// return nil. This case is treated the same way downstream as a healthy signal.
	return nil
}
