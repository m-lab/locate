package instancetest

import v2 "github.com/m-lab/locate/api/v2"

type FakeDatastoreClient struct{}

func (f *FakeDatastoreClient) Put(key string, field string, value interface{}) error {
	return nil
}

func (f *FakeDatastoreClient) Update(key string, field string, value interface{}) error {
	return nil
}

func (f *FakeDatastoreClient) GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error) {
	return map[string]*v2.HeartbeatMessage{}, nil
}
