package memorystore

import (
	"encoding/json"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
)

const (
	// This is a Lua script that will be interpreted by the Redis server.
	// The key/argument parameters (e.g., KEYS[1]) are passed to the script
	// when it is invoked in the Put method (e.g., redis.Args{}.Add(...)).
	// The command used to interpret the script in Redis is the EVAL command.
	// Its documentation can be found under https://redis.io/commands/eval/.
	script = `if redis.call('HGET', KEYS[1], ARGV[1]) == 1
		then return redis.call('HSET', KEYS[1], ARGV[2], ARGV[3])
		else error('key not found')
		end`
)

// PutOptions defines the parameters that can be used for PUT operations.
type PutOptions struct {
	FieldMustExist  string // Specifies a field that must already exist in the entry.
	WithExpire bool   // Specifies whether an expiration should be added to the entry.
}

type client[V any] struct {
	pool *redis.Pool
}

// NewClient returns a new MemorystoreClient implementation
// that reads and writes data in Redis.
func NewClient[V any](pool *redis.Pool) *client[V] {
	return &client[V]{pool}
}

// Put sets a Redis Hash using the `HSET key field value` command.
// If the `opts.WithExpire` option is true, it also (re)sets the key's timeout.
func (c *client[V]) Put(key string, field string, value redis.Scanner, opts *PutOptions) error {
	t := time.Now()
	conn := c.pool.Get()
	defer conn.Close()

	b, err := json.Marshal(value)
	if err != nil {
		metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field, "marshal error").Observe(time.Since(t).Seconds())
		return err
	}

	if opts.FieldMustExist != "" {
		args := redis.Args{}.Add(script).Add(1).Add(key).Add(opts.FieldMustExist).Add(field).AddFlat(string(b))
		_, err = conn.Do("EVAL", args...)
		if err != nil {
			metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field, "EVAL error").Observe(time.Since(t).Seconds())
			return err
		}
	} else {
		args := redis.Args{}.Add(key).Add(field).AddFlat(string(b))
		_, err = conn.Do("HSET", args...)
		if err != nil {
			metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field, "HSET error").Observe(time.Since(t).Seconds())
			return err
		}
	}

	if !opts.WithExpire {
		metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field, "OK").Observe(time.Since(t).Seconds())
		return nil
	}

	_, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	if err != nil {
		metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field, "EXPIRE error").Observe(time.Since(t).Seconds())
		return err
	}

	metrics.LocateMemorystoreRequestDuration.WithLabelValues("put", field+" with expiration", "OK").Observe(time.Since(t).Seconds())
	return nil
}

// GetAll uses the SCAN command to iterate over all the entries in Redis
// and returns a mapping of all the keys to their values.
// It implements an "all or nothing" approach in which it will only
// return the entries if all of them are scanned successfully.
// Otherwise, it will return an error.
func (c *client[V]) GetAll() (map[string]V, error) {
	t := time.Now()
	conn := c.pool.Get()
	defer conn.Close()

	values := make(map[string]V)
	iter := 0

	for {
		keys, err := redis.Values(conn.Do("SCAN", iter))
		if err != nil {
			metrics.LocateMemorystoreRequestDuration.WithLabelValues("get", "all", "SCAN error").Observe(time.Since(t).Seconds())
			return nil, err
		}

		var temp []string
		keys, err = redis.Scan(keys, &iter, &temp)
		if err != nil {
			metrics.LocateMemorystoreRequestDuration.WithLabelValues("get", "all", "SCAN copy error").Observe(time.Since(t).Seconds())
			return nil, err
		}

		for _, k := range temp {
			v, err := c.get(k, conn)
			if err != nil {
				metrics.LocateMemorystoreRequestDuration.WithLabelValues("get", "all", "HGETALL error").Observe(time.Since(t).Seconds())
				return nil, err
			}
			values[k] = v
		}

		if iter == 0 {
			metrics.LocateMemorystoreRequestDuration.WithLabelValues("get", "all", "OK").Observe(time.Since(t).Seconds())
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
