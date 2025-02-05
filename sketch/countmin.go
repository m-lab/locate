package sketch

import (
	"context"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// CMSketch implements the Sketch interface using Redis as a backend
type CMSketch struct {
	config Config
	client *redis.Client
	// used for testing
	nowFunc func() time.Time
}

// New creates a new CMSketch with the given configuration and Redis client
func New(config Config, client *redis.Client) *CMSketch {
	return &CMSketch{
		config: config,
		client: client,
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

	// Use pipeline for batch operation
	pipe := s.client.Pipeline()

	// Increment each counter and set expiration
	for i, hash := range hashes {
		key := s.getCounterKey(windowKey, i)
		pipe.HIncrBy(ctx, key, strconv.FormatUint(hash, 10), 1)
		pipe.Expire(ctx, key, s.config.Window*2) // Keep one extra window for sliding
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *CMSketch) Count(ctx context.Context, item string) (int64, error) {
	windowKey := s.getCurrentWindowKey()
	hashes := s.hash(item)

	var minCount int64 = -1

	// Use pipeline for batch operation
	pipe := s.client.Pipeline()

	// Queue up all counter reads
	cmds := make([]*redis.StringCmd, len(hashes))
	for i, hash := range hashes {
		key := s.getCounterKey(windowKey, i)
		cmds[i] = pipe.HGet(ctx, key, strconv.FormatUint(hash, 10))
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, err
	}

	// Find minimum count
	for _, cmd := range cmds {
		count, err := cmd.Int64()
		if err == redis.Nil {
			count = 0
		} else if err != nil {
			return 0, err
		}

		if minCount == -1 || count < minCount {
			minCount = count
		}
	}

	if minCount == -1 {
		minCount = 0
	}

	return minCount, nil
}

func (s *CMSketch) Reset(ctx context.Context) error {
	windowKey := s.getCurrentWindowKey()

	pipe := s.client.Pipeline()

	// Delete all counters for current window
	for i := 0; i < s.config.Depth; i++ {
		key := s.getCounterKey(windowKey, i)
		pipe.Del(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	return err
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
