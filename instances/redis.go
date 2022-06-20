package instances

import (
	"encoding/json"
	"errors"

	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var errKeyNotFound = errors.New("failed to find key in Redis")

type redisDatastoreClient struct {
	pool *redis.Pool
}

// NewRedisDatastoreClient returns a new DatastoreClient implementation
// that reads and writes data in Redis.
func NewRedisDatastoreClient(pool *redis.Pool) *redisDatastoreClient {
	return &redisDatastoreClient{pool}
}

// Put sets a Redis Hash using the `HSET key field value` command.
// If successful, it also sets a timeout on the key.
func (rc *redisDatastoreClient) Put(key string, field string, value interface{}) error {
	conn := rc.pool.Get()
	defer conn.Close()

	b, err := json.Marshal(value)
	if err != nil {
		return err
	}

	args := redis.Args{}.Add(key).Add(field).AddFlat(string(b))
	_, err = conn.Do("HSET", args...)
	if err == nil {
		_, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	}
	return err
}

// Update updates a field for a Redis Hash if an entry for the key already
// exists. If successful, it resets the timeout on the key.
func (rc *redisDatastoreClient) Update(key string, field string, value interface{}) error {
	conn := rc.pool.Get()
	defer conn.Close()

	ok, err := redis.Bool(conn.Do("EXISTS", key))
	if !ok || err != nil {
		return errKeyNotFound
	}
	return rc.Put(key, field, value)
}

// GetAllHeartbeats uses the SCAN command to iterate over all the entries in Redis
// and return a mapping of all the keys to their v2.HeartbeatMessage values.
func (rc *redisDatastoreClient) GetAllHeartbeats() (values map[string]*v2.HeartbeatMessage, err error) {
	conn := rc.pool.Get()
	defer conn.Close()

	var keys []interface{}
	values = make(map[string]*v2.HeartbeatMessage)
	iter := 0

	for {
		keys, err = redis.Values(conn.Do("SCAN", iter))
		if err != nil {
			return
		}

		var temp []string
		keys, err = redis.Scan(keys, &iter, &temp)

		if err == nil {
			for _, k := range temp {
				hbm, err := getHeartbeat(k, conn)
				if err == nil {
					values[k] = hbm
				}
			}
		}

		if iter == 0 {
			return
		}
	}
}

func getHeartbeat(key string, conn redis.Conn) (*v2.HeartbeatMessage, error) {
	val, err := redis.Values(conn.Do("HGETALL", key))
	if err != nil {
		return nil, err
	}

	hbm := &v2.HeartbeatMessage{}
	err = redis.ScanStruct(val, hbm)
	if err != nil {
		return nil, err
	}

	return hbm, nil
}
