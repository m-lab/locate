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

// Config holds configuration for the rate limiter
type Config struct {
	// RequestsPerMinute is the maximum number of requests allowed per minute
	RequestsPerMinute int64

	// SketchWidth is the width of the Count-Min Sketch
	SketchWidth int

	// SketchDepth is the depth of the Count-Min Sketch
	SketchDepth int
}

// Limiter implements the RateLimiter interface using a Count-Min Sketch
type Limiter struct {
	config Config
	sketch sketch.Sketch
}

// New creates a new rate limiter with the given configuration
func New(config Config, sketch sketch.Sketch) *Limiter {
	return &Limiter{
		config: config,
		sketch: sketch,
	}
}

func (l *Limiter) Allow(ctx context.Context, ip string) (bool, error) {
	// First increment the counter for this IP
	err := l.sketch.Increment(ctx, ip)
	if err != nil {
		// Allow the request but propagate the error so it can be monitored/logged
		return true, err // Changed from false to true
	}

	// Get the current count for this IP
	count, err := l.sketch.Count(ctx, ip)
	if err != nil {
		// Same here - fail open but propagate the error
		return true, err // Changed from false to true
	}

	// Allow if count is within limit
	return count <= l.config.RequestsPerMinute, nil
}
