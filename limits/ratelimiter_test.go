package limits

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/gomodule/redigo/redis"
)

func TestRateLimiter_IsLimited(t *testing.T) {
	// Start miniredis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", s.Addr())
		},
	}

	cleanRedis := func() {
		conn := pool.Get()
		defer conn.Close()
		conn.Do("FLUSHDB")
	}

	// Clean up before all tests
	cleanRedis()

	// Clean up after all tests
	defer cleanRedis()

	t.Run("under limit", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			Interval:  time.Hour,
			MaxEvents: 60,
			KeyPrefix: "test:",
		})

		limited, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("IsLimited() = true, want false")
		}
	})

	t.Run("at limit", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
			KeyPrefix: "test:",
		})

		// First event
		limited, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("first event: IsLimited() = true, want false")
		}

		// Second event (should still be allowed)
		limited, err = rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("second event: IsLimited() = true, want false")
		}

		// Third event (should be limited)
		limited, err = rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if !limited {
			t.Error("third event: IsLimited() = false, want true")
		}
	})

	t.Run("different ip not limited", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
			KeyPrefix: "test:",
		})

		// Add events for first IP
		for i := 0; i < 2; i++ {
			_, err := rl.IsLimited("192.0.2.1", "test-agent")
			if err != nil {
				t.Fatalf("IsLimited() error = %v", err)
			}
		}

		// Different IP should not be limited
		limited, err := rl.IsLimited("192.0.2.2", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("IsLimited() = true, want false")
		}
	})

	t.Run("different ua not limited", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
			KeyPrefix: "test:",
		})

		// Add events for first UA
		for i := 0; i < 2; i++ {
			_, err := rl.IsLimited("192.0.2.1", "test-agent-1")
			if err != nil {
				t.Fatalf("IsLimited() error = %v", err)
			}
		}

		// Different UA should not be limited
		limited, err := rl.IsLimited("192.0.2.1", "test-agent-2")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("IsLimited() = true, want false")
		}
	})

	t.Run("old events expire", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			Interval:  100 * time.Millisecond, // Short interval for testing
			MaxEvents: 2,
			KeyPrefix: "test:",
		})

		// First event
		limited, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("first event: IsLimited() = true, want false")
		}

		// Wait for event to expire
		time.Sleep(200 * time.Millisecond)

		// Should not be limited after expiration
		limited, err = rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if limited {
			t.Error("IsLimited() = true, want false")
		}
	})
	t.Run("redis errors", func(t *testing.T) {
		s.FlushDB()

		// Create a pool that will fail
		failPool := &redis.Pool{
			MaxIdle:     3,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				// Use miniredis but close the connection immediately to simulate errors
				conn, err := redis.Dial("tcp", s.Addr())
				if err != nil {
					return nil, err
				}
				conn.Close()
				return conn, nil
			},
		}

		rl := NewRateLimiter(failPool, RateLimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
			KeyPrefix: "test:",
		})

		_, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err == nil {
			t.Error("IsLimited() with closed connection, want error, got nil")
		}
	})
}
