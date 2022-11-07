package memorystore

import (
	"encoding/json"

	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/locate/static"
)

type client[V any] struct {
	pool *redis.Pool
}

// NewClient returns a new MemorystoreClient implementation
// that reads and writes data in Redis.
func NewClient[V any](pool *redis.Pool) *client[V] {
	return &client[V]{pool}
}

// Put sets a Redis Hash using the `HSET key field value` command.
// If successful, it also sets a timeout on the key.
func (c *client[V]) Put(key string, field string, value redis.Scanner, expire bool) error {
	conn := c.pool.Get()
	defer conn.Close()

	b, err := json.Marshal(value)
	if err != nil {
		return err
	}

	args := redis.Args{}.Add(key).Add(field).AddFlat(string(b))
	_, err = conn.Do("HSET", args...)
	if expire && err == nil {
		_, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	}
	return err
}

// GetAll uses the SCAN command to iterate over all the entries in Redis
// and returns a mapping of all the keys to their values.
// It implements an "all or nothing" approach in which it will only
// return the entries if all of them are scanned successfully.
// Otherwise, it will return an error.
func (c *client[V]) GetAll() (map[string]V, error) {
	conn := c.pool.Get()
	defer conn.Close()

	values := make(map[string]V)
	iter := 0

	for {
		keys, err := redis.Values(conn.Do("SCAN", iter))
		if err != nil {
			return nil, err
		}

		var temp []string
		keys, err = redis.Scan(keys, &iter, &temp)
		if err != nil {
			return nil, err
		}

		for _, k := range temp {
			v, err := c.get(k, conn)
			if err != nil {
				return nil, err
			}
			values[k] = v
		}

		if iter == 0 {
			return values, nil
		}
	}
}

func (c *client[V]) get(key string, conn redis.Conn) (V, error) {
	v := new(V)
	val, err := redis.Values(conn.Do("HGETALL", key))
	if err != nil {
		return *v, err
	}

	err = redis.ScanStruct(val, v)
	if err != nil {
		return *v, err
	}

	return *v, nil
}
