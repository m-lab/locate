package instances

import (
	"sync"

	v2 "github.com/m-lab/locate/api/v2"
)

type instanceManager struct {
	instances map[string]*v2.HeartbeatMessage
	mu        sync.RWMutex
	RedisClient
}

type RedisClient interface {
	AddEntry(key string, value v2.HeartbeatMessage) error
	Update(key string, value v2.Health) error
}

func NewInstanceManager(client RedisClient) *instanceManager {
	return &instanceManager{
		instances:   make(map[string]*v2.HeartbeatMessage),
		RedisClient: client,
	}
}

func (m *instanceManager) RegisterInstance(hbm v2.HeartbeatMessage) error {
	hostname := hbm.Registration.Hostname
	m.registerInstance(hostname, hbm)
	return m.AddEntry(hostname, hbm)
}

func (m *instanceManager) UpdateHealth(hostname string, hm v2.Health) error {
	m.updateHealth(hostname, hm)
	m.Update(hostname, hm)
	return nil
}

func (m *instanceManager) GetAll() []v2.HeartbeatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	i := make([]v2.HeartbeatMessage, 0, len(m.instances))
	for _, val := range m.instances {
		i = append(i, *val.Clone())
	}
	return i
}

func (m *instanceManager) registerInstance(hostname string, hbm v2.HeartbeatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[hostname] = &hbm
}

func (m *instanceManager) updateHealth(hostname string, hm v2.Health) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if instance, found := m.instances[hostname]; found {
		instance.Health = &hm
	}
}
