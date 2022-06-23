package instances

import (
	"fmt"

	"github.com/gomodule/redigo/redis"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
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

func (rc *redisClient) SetHash(key string, value v2.HeartbeatMessage) error {
	conn := rc.pool.Get()
	defer conn.Close()

	args := redis.Args{}.Add(key).AddFlat(value)
	fmt.Printf("HSET args: %+v\n", args)
	reply, err := conn.Do("HSET", args...)
	fmt.Printf("HSET SET reply: %+v, err: %+v\n", reply, err)
	if err == nil {
		reply, err := conn.Do("EXPIRE", key, static.RedisKeyExpirySecs)
		fmt.Printf("HSET EXPIRE reply: %+v, err: %+v\n", reply, err)
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
