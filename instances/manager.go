package instances

import (
	"fmt"
	"sync"

	v2 "github.com/m-lab/locate/api/v2"
)

type instanceManager struct {
	instances map[string]*v2.HeartbeatMessage
	mu        sync.RWMutex
	dsc       DatastoreClient
}

type DatastoreClient interface {
	AddEntry(key string, value v2.HeartbeatMessage) (interface{}, error)
	UpdateHealth(key string, value v2.Health) (interface{}, error)
	GetAll() ([]v2.HeartbeatMessage, error)
}

func NewInstanceManager(client DatastoreClient) *instanceManager {
	return &instanceManager{
		instances: make(map[string]*v2.HeartbeatMessage),
		dsc:       client,
	}
}

func (m *instanceManager) RegisterInstance(hbm v2.HeartbeatMessage) error {
	hostname := hbm.Registration.Hostname
	if _, err := m.dsc.AddEntry(hostname, hbm); err != nil {
		return err
	}
	m.registerInstance(hostname, hbm)
	return nil
}

func (m *instanceManager) HandleHeartbeat(hostname string, hm v2.Health) error {
	if _, err := m.dsc.UpdateHealth(hostname, hm); err != nil {
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
