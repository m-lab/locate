package instancetest

import v2 "github.com/m-lab/locate/api/v2"

type FakeRedisClient struct{}

func (f *FakeRedisClient) AddEntry(key string, value v2.HeartbeatMessage) error {
	return nil
}

func (f *FakeRedisClient) Update(key string, value v2.Health) error {
	return nil
}
