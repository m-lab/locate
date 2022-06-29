package v2

import (
	"encoding/json"
	"fmt"
)

func (r *Registration) RedisScan(x interface{}) error {
	v, ok := x.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %T to []byte", x)
	}
	return json.Unmarshal(v, r)
}

func (h *Health) RedisScan(x interface{}) error {
	v, ok := x.([]byte)
	if !ok {
		return fmt.Errorf("failed to convert %T to []byte", x)
	}
	return json.Unmarshal(v, h)
}
