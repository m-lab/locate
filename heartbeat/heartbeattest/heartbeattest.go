package heartbeattest

import (
	"errors"

	v2 "github.com/m-lab/locate/api/v2"
)

var (
	// FakeMemorystoreClient provides an implementation of MemorystoreClient for testing.
	FakeMemorystoreClient = fakeMemorystoreClient[v2.HeartbeatMessage]{
		m: make(map[string]v2.HeartbeatMessage),
	}
	// FakeErrorMemorystoreClient provides an implementation that returns errors
	// for all its methods.
	FakeErrorMemorystoreClient = fakeErrorMemorystoreClient[v2.HeartbeatMessage]{}
	// FakeError defines an error to be returned by the implementation of FakeErrorMemorystoreClient.
	FakeError = errors.New("error for FakeErrorMemorystoreClient")
)

type fakeMemorystoreClient[V any] struct {
	m map[string]V
}

// Put returns nil.
func (c *fakeMemorystoreClient[V]) Put(key string, field string, value any) error {
	return nil
}

// GetAll returns an empty map and a nil error.
func (c *fakeMemorystoreClient[V]) GetAll() (map[string]V, error) {
	return c.m, nil
}

// FakeAdd mimics adding a new value to Memorystore for testing.
func (c *fakeMemorystoreClient[V]) FakeAdd(key string, value V) {
	c.m[key] = value
}

type fakeErrorMemorystoreClient[V any] struct{}

// Put returns a FakeError.
func (c *fakeErrorMemorystoreClient[V]) Put(key string, field string, value any) error {
	return FakeError
}

// GetAll returns an empty map and a FakeError.
func (c *fakeErrorMemorystoreClient[V]) GetAll() (map[string]V, error) {
	return map[string]V{}, FakeError
}
