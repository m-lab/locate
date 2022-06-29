package instances

import (
	"encoding/json"

	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

type redisDatastoreClient struct {
	pool *redis.Pool
}

func NewRedisDatastoreClient(address string) *redisDatastoreClient {
	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", address)
		},
	}
	return &redisDatastoreClient{redisPool}
}

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

func (rc *redisDatastoreClient) Update(key string, field string, value interface{}) error {
	conn := rc.pool.Get()
	defer conn.Close()

	ok, err := redis.Bool(conn.Do("EXISTS", key))
	if ok {
		err = rc.Put(key, field, value)
	}
	return err
}

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
