package limits

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"
)

var randFloat64 = rand.Float64

// LimitStatus indicates the result of a rate limit check
type LimitStatus struct {
	// IsLimited indicates if the request should be rate limited
	IsLimited bool
	// LimitType indicates which limit was exceeded ("ip" or "ipua" or "")
	LimitType string
}

// LimitConfig holds the configuration for a single rate limit type
type LimitConfig struct {
	// Interval defines the duration of the sliding window
	Interval time.Duration

	// MaxEvents defines the maximum number of events allowed in the interval
	MaxEvents int
}

// RateLimitConfig holds the configuration for both IP-only and IP+UA rate limiting.
type RateLimitConfig struct {
	// IPConfig defines the rate limiting configuration for IP-only checks
	IPConfig LimitConfig

	// IPUAConfig defines the rate limiting configuration for IP+UA checks
	IPUAConfig LimitConfig

	// KeyPrefix is the prefix for Redis keys
	KeyPrefix string
}

// RateLimiter implements a distributed rate limiter using Redis sorted sets (ZSET).
// It maintains sliding windows for both IP-only and IP+UA combinations, where:
//   - Each event is stored in a ZSET with the timestamp as score
//   - Old events (outside the window) are automatically removed
//   - Keys automatically expire after the configured interval
//
// The limiter considers a request to be rate-limited if the number of events
// in either window exceeds their respective MaxEvents.
type RateLimiter struct {
	pool       *redis.Pool
	ipConfig   LimitConfig
	ipuaConfig LimitConfig
	keyPrefix  string
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(pool *redis.Pool, config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		pool:       pool,
		ipConfig:   config.IPConfig,
		ipuaConfig: config.IPUAConfig,
		keyPrefix:  config.KeyPrefix,
	}
}

// generateIPKey creates a Redis key from IP only.
func (rl *RateLimiter) generateIPKey(ip string) string {
	return fmt.Sprintf("%s:%s", rl.keyPrefix, ip)
}

// generateIPUAKey creates a Redis key from IP and User-Agent.
func (rl *RateLimiter) generateIPUAKey(ip, ua string) string {
	// If User-Agent is empty, use "none" as the value. This allows to distinguish
	// between IP-only keys and IPUA keys with an empty User-Agent.
	if ua == "" {
		ua = "none"
	}
	return fmt.Sprintf("%s:%s:%s", rl.keyPrefix, ip, ua)
}

// IsLimited checks if the given IP and User-Agent combination should be rate limited.
// It first checks the IP-only limit, then the IP+UA limit if the IP-only check passes.
func (rl *RateLimiter) IsLimited(ip, ua string) (LimitStatus, error) {
	conn := rl.pool.Get()
	defer conn.Close()

	now := time.Now().UnixMicro()
	ipKey := rl.generateIPKey(ip)
	ipuaKey := rl.generateIPUAKey(ip, ua)

	// Start pipeline for both checks
	// 1. IP-only check
	conn.Send("ZREMRANGEBYSCORE", ipKey, "-inf", now-rl.ipConfig.Interval.Microseconds())
	conn.Send("ZADD", ipKey, now, strconv.FormatInt(now, 10))
	conn.Send("EXPIRE", ipKey, int64(rl.ipConfig.Interval.Seconds()))
	conn.Send("ZCARD", ipKey)

	// 2. IP+UA limit check
	conn.Send("ZREMRANGEBYSCORE", ipuaKey, "-inf", now-rl.ipuaConfig.Interval.Microseconds())
	conn.Send("ZADD", ipuaKey, now, strconv.FormatInt(now, 10))
	conn.Send("EXPIRE", ipuaKey, int64(rl.ipuaConfig.Interval.Seconds()))
	conn.Send("ZCARD", ipuaKey)

	// Flush pipeline
	if err := conn.Flush(); err != nil {
		return LimitStatus{}, fmt.Errorf("failed to flush pipeline: %w", err)
	}

	// Receive first 3 replies for IP limit (ZREMRANGEBYSCORE, ZADD, EXPIRE)
	for i := 0; i < 3; i++ {
		if _, err := conn.Receive(); err != nil {
			return LimitStatus{}, fmt.Errorf("failed to receive IP limit reply %d: %w", i, err)
		}
	}

	// Receive IP limit count
	ipCount, err := redis.Int64(conn.Receive())
	if err != nil {
		return LimitStatus{}, fmt.Errorf("failed to receive IP limit count: %w", err)
	}

	// Check IP-only limit first.
	if ipCount > int64(rl.ipConfig.MaxEvents) {
		// Allow 1/(count) requests to get through the limiter anyway.
		// This is effectively one request per IP every interval.
		if randFloat64() >= 1.0/float64(ipCount) {
			return LimitStatus{
				IsLimited: true,
				LimitType: "ip",
			}, nil
		}
	}

	// Receive next 3 replies for IP+UA limit (ZREMRANGEBYSCORE, ZADD, EXPIRE)
	for i := 0; i < 3; i++ {
		if _, err := conn.Receive(); err != nil {
			return LimitStatus{}, fmt.Errorf("failed to receive IP+UA limit reply %d: %w", i, err)
		}
	}

	// Receive IP+UA limit count
	ipuaCount, err := redis.Int64(conn.Receive())
	if err != nil {
		return LimitStatus{}, fmt.Errorf("failed to receive IP+UA limit count: %w", err)
	}

	// Check IP+UA limit
	if ipuaCount > int64(rl.ipuaConfig.MaxEvents) {
		// Allow 1/(count) requests to get through the limiter anyway.
		// This is effectively one request per IP+UA every interval.
		if randFloat64() >= 1.0/float64(ipuaCount) {
			return LimitStatus{
				IsLimited: true,
				LimitType: "ipua",
			}, nil
		}
	}

	return LimitStatus{
		IsLimited: false,
		LimitType: "",
	}, nil
}
