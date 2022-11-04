package heartbeat

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
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

	h.registerInstanceLocally(hostname, rm)
	return nil
}

// UpdateHealth updates the v2.Health field for the instance in the Memorystore client and
// updates it locally.
func (h *heartbeatStatusTracker) UpdateHealth(hostname string, hm v2.Health) error {
	if err := h.Put(hostname, "Health", &hm, true); err != nil {
		return err
	}
	return h.updateHealthLocally(hostname, hm)
}

// UpdatePrometheus updates the v2.Prometheus field for all instances in Memorystore and
// locally.
func (h *heartbeatStatusTracker) UpdatePrometheus(hostnames, machines map[string]bool) error {
	var err error

	for hostname, instance := range h.instances {
		pm := getPrometheusMessage(instance, hostnames, machines)
		if pm != nil {
			// Update in Memorystore.
			memorystoreErr := h.Put(hostname, "Prometheus", pm, false)
			if memorystoreErr != nil {
				err = errPrometheus
				continue
			}

			// Update locally.
			h.updatePrometheusLocally(instance, pm)
		}
	}

	log.Printf("UpdatePrometheus: hostnames %+v", hostnames)
	log.Printf("UpdatePrometheus: machines %+v", machines)

	for h, i := range h.instances {
		log.Printf("UpdatePrometheus: registration %s %+v", h, i.Registration)
		log.Printf("UpdatePrometheus: instances %s %+v", h, i.Prometheus)
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

func (h *heartbeatStatusTracker) registerInstanceLocally(hostname string, rm v2.Registration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.instances[hostname] = v2.HeartbeatMessage{Registration: &rm}
}

func (h *heartbeatStatusTracker) updateHealthLocally(hostname string, hm v2.Health) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if instance, found := h.instances[hostname]; found {
		instance.Health = &hm
		h.instances[hostname] = instance
		return nil
	}

	return fmt.Errorf("failed to find %s instance for health update", hostname)
}

func (h *heartbeatStatusTracker) updatePrometheusLocally(instance v2.HeartbeatMessage, pm *v2.Prometheus) {
	h.mu.Lock()
	defer h.mu.Unlock()

	instance.Prometheus = pm
	hostname := instance.Registration.Hostname
	h.instances[hostname] = instance
}

func (h *heartbeatStatusTracker) importMemorystore() {
	values, err := h.GetAll()

	if err == nil {
		h.instances = values
	}
}

// getPrometheusMessage constructs a v2.Prometheus message for a specific instance
// from a map of hostname/machine Prometheus data.
// If no information is available for the instance, it returns nil.
func getPrometheusMessage(instance v2.HeartbeatMessage, hostnames, machines map[string]bool) *v2.Prometheus {
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
		health := (!hostFound || hostHealthy) && (!machineFound || machineHealthy)
		return &v2.Prometheus{Health: health}
	}

	return nil
}
