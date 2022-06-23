package instances

import (
	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

const (
	updateScript = `if redis.call('exists', KEYS[1]) == 1 then redis.call('hset', KEYS[1], ARGV[1], 0) end`
)

type redisClient struct {
	pool *redis.Pool
}

func NewRedisClient(address string) *redisClient {
	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", address)
		},
	}
	return &redisClient{redisPool}
}

func (rc *redisClient) AddEntry(key string, value v2.HeartbeatMessage) error {
	conn := rc.pool.Get()
	defer conn.Close()

	args := redis.Args{}.Add(key).AddFlat(value)
	_, err := conn.Do("HSET", args...)
	if err == nil {
		_, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	}
	return err
}

func (rc *redisClient) Update(key string, value v2.Health) error {
	conn := rc.pool.Get()
	defer conn.Close()

	lua := redis.NewScript(1, updateScript)
	// TODO: Make value Health.Score
	_, err := lua.Do(conn, key, value)
	if err == nil {
		_, err = conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
	}
	return err
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
