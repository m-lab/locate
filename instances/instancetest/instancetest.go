package instancetest

import v2 "github.com/m-lab/locate/api/v2"

type FakeDatastoreClient struct{}

func (f *FakeDatastoreClient) AddEntry(key string, value v2.HeartbeatMessage) (interface{}, error) {
	return nil, nil
}

func (f *FakeDatastoreClient) UpdateHealth(key string, value v2.Health) error {
	return nil
}

func (f *FakeDatastoreClient) GetAll() ([]v2.HeartbeatMessage, error) {
	return []v2.HeartbeatMessage{}, nil
}
