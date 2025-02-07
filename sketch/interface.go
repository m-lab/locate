package sketch

import (
	"context"
	"time"
)

// Sketch represents a Redis-backed Count-Min Sketch for counting events
type Sketch interface {
	// Increment adds 1 to the counters for the given item.
	Increment(ctx context.Context, item string) error

	// Count returns the estimated count for the given item.
	Count(ctx context.Context, item string) (int, error)
}

// Config holds configuration for the Count-Min Sketch
type Config struct {
	// Width is the number of counters per hash function
	Width int

	// Depth is the number of hash functions
	Depth int

	// Window is the duration of the counting window
	Window time.Duration

	// RedisKeyPrefix is the prefix for all Redis keys used by this sketch
	RedisKeyPrefix string
}
