package redis

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/locate/static"
)

type RedisClient struct {
	pool *redis.Pool
}

func NewRedisClient(address string) *RedisClient {
	redisPool := &redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", address)
		},
	}
	return &RedisClient{redisPool}
}

func (rc *RedisClient) SetHash(key string, value ...interface{}) error {
	conn := rc.pool.Get()
	defer conn.Close()

	args := redis.Args{}.Add(key).AddFlat(value)
	fmt.Printf("HSET args: %+v\n", args)
	_, err := conn.Do("HSET", args...)
	if err == nil {
		_, err = conn.Do("EXPIRE", key, static.RedisKeyExpiry)
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
// }
