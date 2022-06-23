package instances

import (
	"log"
	"sync"

	v2 "github.com/m-lab/locate/api/v2"
)

type instanceManager struct {
	instances map[string]*v2.HeartbeatMessage
	mu        sync.RWMutex
	RedisClient
}

type RedisClient interface {
	SetHash(key string, value v2.Registration) error
}

func NewInstanceManager(client RedisClient) *instanceManager {
	return &instanceManager{
		instances:   make(map[string]*v2.HeartbeatMessage),
		RedisClient: client,
	}
}

func (m *instanceManager) RegisterInstance(rm v2.Registration) {
	m.registerInstance(rm)
	err := m.SetHash(rm.Hostname, rm)
	if err != nil {
		log.Printf("failed to register instance in redis, err: %v", err)
	}
}

func (m *instanceManager) UpdateHealth(hostname string, hm v2.Health) {
	m.updateHealth(hostname, hm)
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

func (m *instanceManager) registerInstance(rm v2.Registration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instances[rm.Hostname] = &v2.HeartbeatMessage{Registration: &rm}
}

func (m *instanceManager) updateHealth(hostname string, hm v2.Health) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if instance, found := m.instances[hostname]; found {
		instance.Health = &hm
	}
}
