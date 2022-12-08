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
	metrics.HeartbeatHealthStatus.WithLabelValues(hostname).Set(hm.Score)
	return h.updateHealth(hostname, hm)
}

// UpdatePrometheus updates the v2.Prometheus field for the instances.
func (h *heartbeatStatusTracker) UpdatePrometheus(hostnames, machines map[string]bool) error {
	var err error

	for _, instance := range h.instances {
		pm := constructPrometheusMessage(instance, hostnames, machines)
		if pm != nil {
			updateErr := h.updatePrometheusMessage(instance, pm)

			if updateErr != nil {
				err = errPrometheus
			}

			status := promNumericStatus(pm)
			metrics.PrometheusHealthStatus.WithLabelValues(instance.Registration.Hostname).Set(status)
		}
	}

	return err
}

// Instances returns a mapping of all the v2.HeartbeatMessage instance keys to
// their values.
func (h *heartbeatStatusTracker) Instances() map[string]v2.HeartbeatMessage {
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
	h.mu.Lock()
	defer h.mu.Unlock()
	instance.Prometheus = pm
	h.instances[hostname] = instance
	return nil
}

func (h *heartbeatStatusTracker) importMemorystore() {
	values, err := h.GetAll()

	if err == nil {
		h.instances = values

		metrics.LocateHealthStatus.Reset()
		for _, instance := range h.instances {
			if instance.Registration != nil {
				metrics.LocateHealthStatus.WithLabelValues(instance.Registration.Experiment).Add(getHealth(instance))
			}
		}
	}
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

func promNumericStatus(pm *v2.Prometheus) float64 {
	if pm.Health {
		return 1
	}
	return 0
}

func getHealth(instance v2.HeartbeatMessage) float64 {
	if instance.Health != nil && instance.Health.Score > 0 &&
		(instance.Prometheus == nil || instance.Prometheus.Health) {
		return 1
	}
	return 0
}
