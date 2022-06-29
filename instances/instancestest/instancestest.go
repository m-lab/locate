package instancestest

import (
	"errors"

	v2 "github.com/m-lab/locate/api/v2"
)

// FakeDatastoreClient provides an empty implementation of DatastoreClient
// for testing.
type FakeDatastoreClient struct{}

// Put returns nil.
func (c *FakeDatastoreClient) Put(key string, field string, value interface{}) error {
	return nil
}

// Update returns nil.
func (c *FakeDatastoreClient) Update(key string, field string, value interface{}) error {
	return nil
}

// GetAllHeartbeats returns an empty map[string]*v2.HeartbeatMessage{} and a nil
// error.
func (c *FakeDatastoreClient) GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error) {
	return map[string]*v2.HeartbeatMessage{}, nil
}

// FakeErrorDatastoreClient provides an implementation that returns errors
// for its methods
type FakeErrorDatastoreClient struct{}

var FakeError = errors.New("error for FakeErrorDatastoreClient")

// Put returns a FakeError.
func (c *FakeErrorDatastoreClient) Put(key string, field string, value interface{}) error {
	return FakeError
}

// Update returns a FakeError.
func (c *FakeErrorDatastoreClient) Update(key string, field string, value interface{}) error {
	return FakeError
}

// GetAllHeartbeats returns an empty map[string]*v2.HeartbeatMessage{} and a FakeError.
func (c *FakeErrorDatastoreClient) GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error) {
	return map[string]*v2.HeartbeatMessage{}, FakeError
}
