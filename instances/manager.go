package instances

import (
	"fmt"
	"sync"
	"time"

	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

type instanceManager struct {
	instances map[string]*v2.HeartbeatMessage
	mu        sync.RWMutex
	dsc       DatastoreClient
	stop      chan bool
}

type DatastoreClient interface {
	Put(key string, field string, value interface{}) error
	Update(key string, field string, value interface{}) error
	GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error)
}

func NewInstanceManager(client DatastoreClient) *instanceManager {
	m := &instanceManager{
		instances: make(map[string]*v2.HeartbeatMessage),
		dsc:       client,
		stop:      make(chan bool),
	}
	go m.consumeDatastore()
	return m
}

func (m *instanceManager) RegisterInstance(hbm v2.HeartbeatMessage) error {
	hostname := hbm.Registration.Hostname
	if err := m.dsc.Put(hostname, "Registration", *hbm.Registration); err != nil {
		return err
	}
	m.registerInstance(hostname, hbm)
	return nil
}

func (m *instanceManager) Stop() {
	m.stop <- true
}

func (m *instanceManager) HandleHeartbeat(hostname string, hm v2.Health) error {
	if err := m.dsc.Update(hostname, "Health", hm); err != nil {
		return err
	}
	return m.updateHealth(hostname, hm)
}

func (m *instanceManager) registerInstance(hostname string, hbm v2.HeartbeatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[hostname] = &hbm
}

func (m *instanceManager) updateHealth(hostname string, hm v2.Health) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if instance, found := m.instances[hostname]; found {
		instance.Health = &hm
		return nil
	}
	return fmt.Errorf("failed to find %s instance for health update", hostname)
}

func (m *instanceManager) consumeDatastore() {
	ticker := *time.NewTicker(static.DatastoreExportPeriod)
	defer ticker.Stop()

	// TODO: add cancel.
	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			values, err := m.dsc.GetAllHeartbeats()
			if err != nil {
				m.instances = values
			}
		}
	}
}
