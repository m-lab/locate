package instancetest

import v2 "github.com/m-lab/locate/api/v2"

type FakeRedisClient struct{}

func (f *FakeRedisClient) SetHash(key string, value v2.HeartbeatMessage) error {
	return nil
}
