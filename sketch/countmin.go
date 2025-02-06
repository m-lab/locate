package sketch

import (
	"context"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
)

// CMSketch implements the Sketch interface using Redis as a backend
type CMSketch struct {
	config Config
	pool   *redis.Pool
	// nowFunc allows overriding time.Now for testing
	nowFunc func() time.Time
}

// New creates a new CMSketch with the given configuration and Redis client
func New(config Config, pool *redis.Pool) *CMSketch {
	return &CMSketch{
		config: config,
		pool:   pool,
	}
}

// hash generates k different hash values for the given item
func (s *CMSketch) hash(item string) []uint64 {
	hashes := make([]uint64, s.config.Depth)
	h := fnv.New64a()

	for i := 0; i < s.config.Depth; i++ {
		h.Reset()
		h.Write([]byte(item))
		h.Write([]byte{byte(i)}) // Add salt for different hashes
		hashes[i] = h.Sum64() % uint64(s.config.Width)
	}

	return hashes
}

func (s *CMSketch) Increment(ctx context.Context, item string) error {
	// Get current time window key
	windowKey := s.getCurrentWindowKey()

	// Get hash values for item
	hashes := s.hash(item)

	// Get connection from pool
	conn := s.pool.Get()
	defer conn.Close()

	// Send all commands to pipeline
	for i, hash := range hashes {
		key := s.getCounterKey(windowKey, i)
		hashStr := strconv.FormatUint(hash, 10)

		conn.Send("HINCRBY", key, hashStr, 1)
		conn.Send("EXPIRE", key, int(s.config.Window.Seconds()*2))
	}

	// Execute pipeline
	return conn.Flush()
}

func (s *CMSketch) Count(ctx context.Context, item string) (int64, error) {
	windowKey := s.getCurrentWindowKey()
	hashes := s.hash(item)

	conn := s.pool.Get()
	defer conn.Close()

	var minCount int64 = -1

	// Send all HGET commands to pipeline
	for i, hash := range hashes {
		key := s.getCounterKey(windowKey, i)
		hashStr := strconv.FormatUint(hash, 10)
		conn.Send("HGET", key, hashStr)
	}

	// Execute pipeline
	if err := conn.Flush(); err != nil {
		return 0, err
	}

	// Receive all responses
	for range hashes {
		value, err := redis.Int64(conn.Receive())
		if err == redis.ErrNil {
			value = 0
		} else if err != nil {
			return 0, err
		}

		if minCount == -1 || value < minCount {
			minCount = value
		}
	}

	if minCount == -1 {
		minCount = 0
	}

	return minCount, nil
}

func (s *CMSketch) Reset(ctx context.Context, item string) error {
	windowKey := s.getCurrentWindowKey()

	conn := s.pool.Get()
	defer conn.Close()

	// Send DEL commands for all counters
	for i := 0; i < s.config.Depth; i++ {
		key := s.getCounterKey(windowKey, i)
		conn.Send("DEL", key)
	}

	return conn.Flush()
}

// Helper methods for key management
func (s *CMSketch) getCurrentWindowKey() string {
	now := time.Now().UTC()
	if s.nowFunc != nil {
		now = s.nowFunc()
	}
	return now.Format("2006-01-02T15:04")
}

func (s *CMSketch) getCounterKey(windowKey string, depth int) string {
	return s.config.RedisKeyPrefix + ":" + windowKey + ":" + strconv.Itoa(depth)
}
