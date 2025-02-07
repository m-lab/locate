package ratelimit

import (
	"context"

	"github.com/m-lab/locate/sketch"
)

// RateLimiter tracks request rates and determines if they exceed limits
type RateLimiter interface {
	// Allow returns true if the request should be allowed, false if it should be rate limited
	Allow(ctx context.Context, ip string) (bool, error)
}

// Limiter implements the RateLimiter interface using a Count-Min Sketch.
type Limiter struct {
	limit  int
	sketch sketch.Sketch
}

// New creates a new rate limiter with the given limit and sketch.
func New(limit int, sketch sketch.Sketch) *Limiter {
	return &Limiter{
		limit:  limit,
		sketch: sketch,
	}
}

func (l *Limiter) Allow(ctx context.Context, ip string) (bool, error) {
	// First increment the counter for this IP
	err := l.sketch.Increment(ctx, ip)
	if err != nil {
		// Allow the request but propagate the error
		return true, err
	}

	// Get the current count for this IP
	count, err := l.sketch.Count(ctx, ip)
	if err != nil {
		// Same here - fail open but propagate the error
		return true, err
	}

	// Allow if count is within limit
	return count <= l.limit, nil
}
