package v2

import (
	"encoding/json"
	"fmt"
)

// RedisScan determines how Registration objects will be interpreted when
// read from Redis.
func (r *Registration) RedisScan(x interface{}) error {
	v, ok := x.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %T to []byte", x)
	}
	return json.Unmarshal(v, r)
}

// RedisScan determines how Health objects will be interpreted when read
// from Redis.
func (h *Health) RedisScan(x interface{}) error {
	v, ok := x.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %T to []byte", x)
	}
	return json.Unmarshal(v, h)
}

// RedisScan determines how Prometheus objects will be interpreted when read
// from Redis.
func (h *Prometheus) RedisScan(x interface{}) error {
	v, ok := x.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %T to []byte", x)
	}
	return json.Unmarshal(v, h)
}
