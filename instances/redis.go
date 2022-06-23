package instances

import (
	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

const (
	updateScript = `
	if redis.call('exists', KEYS[1]) == 1 then
		redis.call('hset', KEYS[1], ARGV[1], 0)
		redis.call('expire', KEYS[1], ARGV[2])
	end`
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

func (rc *redisDatastoreClient) AddEntry(key string, value v2.HeartbeatMessage) (interface{}, error) {
	conn := rc.pool.Get()
	defer conn.Close()

	args := redis.Args{}.Add(key).AddFlat(value)
	reply, err := conn.Do("HSET", args...)
	if err == nil {
		reply, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	}
	return reply, err
}

func (rc *redisDatastoreClient) UpdateHealth(key string, value v2.Health) (interface{}, error) {
	conn := rc.pool.Get()
	defer conn.Close()

	lua := redis.NewScript(1, updateScript)
	reply, err := lua.Do(conn, key, value, static.RedisKeyExpirySecs)
	return reply, err
}

func (rc *redisDatastoreClient) GetAll() ([]v2.HeartbeatMessage, error) {
	return nil, nil
}

// func (rc *RedisClient) GetAll() ([]interface{}, error) {
// 	conn := rc.pool.Get()
// 	defer conn.Close()

// 	iter := 0
// 	keys := make([]interface{}, 0)
// 	iter := 0

// 	for {
// 		val, err := redis.Values(conn.Do("SCAN", iter))
// 		if err != nil {
// 			return nil, err
// 		}

// 		val, err = redis.Scan(val, &iter, &)

// 		if iter == 0 {
// 			return entries, nil
// 		}
// 	}
//
