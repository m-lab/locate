package heartbeattest

import (
	"errors"

	v2 "github.com/m-lab/locate/api/v2"
)

var (
	// FakeDatastoreClient provides an implementation of DatastoreClient for testing.
	FakeDatastoreClient = fakeDatastoreClient[v2.HeartbeatMessage]{
		m: make(map[string]v2.HeartbeatMessage),
	}
	// FakeErrorDatastoreClient provides an implementation that returns errors
	// for all its methods.
	FakeErrorDatastoreClient = fakeErrorDatastoreClient[v2.HeartbeatMessage]{}
	// FakeError defines an error to be returned by the implementation of FakeErrorDatastoreClient.
	FakeError = errors.New("error for FakeErrorDatastoreClient")
)

type fakeDatastoreClient[V any] struct {
	m map[string]V
}

// Put returns nil.
func (c *fakeDatastoreClient[V]) Put(key string, field string, value any) error {
	return nil
}

// GetAll returns an empty map and a nil error.
func (c *fakeDatastoreClient[V]) GetAll() (map[string]V, error) {
	return c.m, nil
}

// FakeAdd mimics adding a new v2.HeartbeatMessage to the datastore for testing.
func (c *fakeDatastoreClient[V]) FakeAdd(key string, value V) {
	c.m[key] = value
}

type fakeErrorDatastoreClient[V any] struct{}

// Put returns a FakeError.
func (c *fakeErrorDatastoreClient[V]) Put(key string, field string, value any) error {
	return FakeError
}

// GetAll returns an empty map and a FakeError.
func (c *fakeErrorDatastoreClient[V]) GetAll() (map[string]V, error) {
	return map[string]V{}, FakeError
}
