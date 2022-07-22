package heartbeat

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var errInvalidArgument = errors.New("argument is invalid")

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
	Put(key string, field string, value redis.Scanner) error
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
	if err := h.Put(hostname, "Registration", &rm); err != nil {
		return err
	}

	h.registerInstance(hostname, rm)
	return nil
}

// UpdateHealth updates the v2.Health field for the instance in the Memorystore client and
// updates it locally.
func (h *heartbeatStatusTracker) UpdateHealth(hostname string, hm v2.Health) error {
	if err := h.Put(hostname, "Health", &hm); err != nil {
		return err
	}
	return h.updateHealth(hostname, hm)
}

// Instances returns a mapping of all the v2.HeartbeatMessage instance keys to
// their values.
func (h *heartbeatStatusTracker) Instances() map[string]v2.HeartbeatMessage {
	return h.instances
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

func (h *heartbeatStatusTracker) importMemorystore() {
	values, err := h.GetAll()

	if err == nil {
		h.instances = values
	}
}
