package instancestest

import (
	"errors"

	v2 "github.com/m-lab/locate/api/v2"
)

// FakeDatastoreClient provides an implementation of DatastoreClient for testing.
type FakeDatastoreClient struct {
	m map[string]*v2.HeartbeatMessage
}

// Put returns nil.
func (c *FakeDatastoreClient) Put(key string, field string, value interface{}) error {
	return nil
}

// Update returns nil
func (c *FakeDatastoreClient) Update(key string, field string, value interface{}) error {
	return nil
}

// GetAllHeartbeats returns the map of entries and a nil error.
func (c *FakeDatastoreClient) GetAllHeartbeats() (map[string]*v2.HeartbeatMessage, error) {
	return c.m, nil
}

// FakeAdd mimics adding a new v2.HeartbeatMessage to the datastore for testing.
func (c *FakeDatastoreClient) FakeAdd(key string, value *v2.HeartbeatMessage) {
	if c.m == nil {
		c.m = make(map[string]*v2.HeartbeatMessage)
	}
	c.m[key] = value
}

// FakeErrorDatastoreClient provides an implementation that returns errors
// for all its methods.
type FakeErrorDatastoreClient struct{}

// FakeError defines an error to be return by the implementation of FakeErrorDatastoreClient.
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
