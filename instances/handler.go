package instances

import (
	"errors"
	"fmt"
	"sync"
	"time"

	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var errInvalidArgument = errors.New("argument is invalid")

type cachingInstanceHandler struct {
	DatastoreClient
	instances map[string]*v2.HeartbeatMessage
	mu        sync.RWMutex
	stop      chan bool
}

// DatastoreClient is a client for reading and writing data in a datastore.
type DatastoreClient interface {
	Put(key string, field string, value interface{}) error
	Update(key string, field string, value interface{}) error
	// TODO(cristinaleon): Try to transform this method into a more generic
	// version (e.g., GetAll(dst ...interface{})).
	GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error)
}

// NewCachingInstanceHandler returns a new InstanceHandler implementation that uses
// a datastore to cache (and later import) instance data.
func NewCachingInstanceHandler(client DatastoreClient) *cachingInstanceHandler {
	h := &cachingInstanceHandler{
		DatastoreClient: client,
		instances:       make(map[string]*v2.HeartbeatMessage),
		stop:            make(chan bool),
	}

	// Start import loop.
	go func(h *cachingInstanceHandler) {
		ticker := *time.NewTicker(static.DatastoreExportPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-h.stop:
				return
			case <-ticker.C:
				h.importDatastore()
			}
		}
	}(h)

	return h
}

// RegisterInstance adds a new v2.Registration message to the datastore and keeps it
// locally.
func (h *cachingInstanceHandler) RegisterInstance(hbm v2.HeartbeatMessage) error {
	if hbm.Registration == nil {
		return errInvalidArgument
	}

	hostname := hbm.Registration.Hostname
	if err := h.Put(hostname, "Registration", *hbm.Registration); err != nil {
		return err
	}

	h.registerInstance(hostname, hbm)
	return nil
}

// UpdateHealth updates the v2.Health field for the instance in the datastore and
// updates it locally.
func (h *cachingInstanceHandler) UpdateHealth(hostname string, hm v2.Health) error {
	if err := h.Update(hostname, "Health", hm); err != nil {
		return err
	}
	return h.updateHealth(hostname, hm)
}

// StopImport stops importing instance data from the Datastore.
func (h *cachingInstanceHandler) StopImport() {
	h.stop <- true
}

func (h *cachingInstanceHandler) registerInstance(hostname string, hbm v2.HeartbeatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.instances[hostname] = &hbm
}

func (h *cachingInstanceHandler) updateHealth(hostname string, hm v2.Health) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if instance, found := h.instances[hostname]; found {
		instance.Health = &hm
		return nil
	}
	return fmt.Errorf("failed to find %s instance for health update", hostname)
}

func (h *cachingInstanceHandler) importDatastore() {
	values, err := h.GetAllHeartbeats()
	if err == nil {
		h.instances = values
	}
}
