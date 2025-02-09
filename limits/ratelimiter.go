package limits

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
)

// RateLimitConfig holds the configuration for IP+UA rate limiting
type RateLimitConfig struct {
	Interval  time.Duration
	MaxEvents int
	KeyPrefix string
}

// RateLimiter implements a distributed rate limiter using Redis sorted sets (ZSET).
// It maintains a sliding window of events for each IP+UA combination, where:
//   - Each event is stored in a ZSET with the timestamp as score
//   - Old events (outside the window) are automatically removed
//   - Keys automatically expire after the configured interval
//
// The limiter considers a request to be rate-limited if the number of events
// in the current window exceeds MaxEvents.
type RateLimiter struct {
	pool      *redis.Pool
	interval  time.Duration
	maxEvents int
	keyPrefix string
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(pool *redis.Pool, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		pool:      pool,
		interval:  config.Interval,
		maxEvents: config.MaxEvents,
		keyPrefix: config.KeyPrefix,
	}
}

// generateKey creates a Redis key from IP and User-Agent
func (rl *RateLimiter) generateKey(ip, ua string) string {
	return fmt.Sprintf("%s%s:%s", rl.keyPrefix, ip, ua)
}

// IsLimited checks if the given IP and User-Agent combination should be rate limited.
func (rl *RateLimiter) IsLimited(ip, ua string) (bool, error) {
	conn := rl.pool.Get()
	defer conn.Close()

	now := time.Now().UnixMicro()
	windowStart := now - rl.interval.Microseconds()
	redisKey := rl.generateKey(ip, ua)

	// Send all commands in pipeline.
	// 1. Remove events outside the window
	conn.Send("ZREMRANGEBYSCORE", redisKey, "-inf", windowStart)
	// 2. Add current event
	conn.Send("ZADD", redisKey, now, strconv.FormatInt(now, 10))
	// 3. Set key expiration
	conn.Send("EXPIRE", redisKey, int64(rl.interval.Seconds()))
	// 4. Get total event count
	conn.Send("ZCARD", redisKey)

	// Flush pipeline
	if err := conn.Flush(); err != nil {
		return false, fmt.Errorf("failed to flush pipeline: %w", err)
	}

	// Receive all replies
	for i := 0; i < 3; i++ {
		// Receive replies for ZREMRANGEBYSCORE, ZADD, and EXPIRE
		if _, err := conn.Receive(); err != nil {
			return false, fmt.Errorf("failed to receive reply %d: %w", i, err)
		}
	}

	// Receive and process ZCARD reply
	count, err := redis.Int64(conn.Receive())
	if err != nil {
		return false, fmt.Errorf("failed to receive count: %w", err)
	}

	return count > int64(rl.maxEvents), nil
}
