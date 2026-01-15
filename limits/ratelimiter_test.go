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

	t.Run("under both limits", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 60,
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 30,
			},
			KeyPrefix: "test:",
		})

		status, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("IsLimited() = %+v, want not limited", status)
		}
	})

	t.Run("ip limit exceeded", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 10, // Higher than IP limit
			},
			KeyPrefix: "test:",
		})

		// First two requests (should be allowed)
		for i := 0; i < 2; i++ {
			status, err := rl.IsLimited("192.0.2.1", "test-agent")
			if err != nil {
				t.Fatalf("IsLimited() error = %v", err)
			}
			if status.IsLimited {
				t.Errorf("request %d: IsLimited() = %+v, want not limited", i, status)
			}
		}

		// Third request (should be IP-limited)
		status, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if !status.IsLimited {
			t.Error("IsLimited() = not limited, want limited")
		}
		if status.LimitType != "ip" {
			t.Errorf("LimitType = %s, want ip", status.LimitType)
		}
	})

	t.Run("ipua limit exceeded", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 10, // Higher than IP+UA limit
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			KeyPrefix: "test:",
		})

		// First two requests (should be allowed)
		for i := 0; i < 2; i++ {
			status, err := rl.IsLimited("192.0.2.1", "test-agent")
			if err != nil {
				t.Fatalf("IsLimited() error = %v", err)
			}
			if status.IsLimited {
				t.Errorf("request %d: IsLimited() = %+v, want not limited", i, status)
			}
		}

		// Third request (should be IP+UA-limited)
		status, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if !status.IsLimited {
			t.Error("IsLimited() = not limited, want limited")
		}
		if status.LimitType != "ipua" {
			t.Errorf("LimitType = %s, want ipua", status.LimitType)
		}
	})

	t.Run("different ip not limited", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
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
		status, err := rl.IsLimited("192.0.2.2", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("IsLimited() = %+v, want not limited", status)
		}
	})

	t.Run("old events expire", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  100 * time.Millisecond,
				MaxEvents: 2,
			},
			IPUAConfig: LimitConfig{
				Interval:  100 * time.Millisecond,
				MaxEvents: 2,
			},
			KeyPrefix: "test:",
		})

		// First event
		status, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("first event: IsLimited() = %+v, want not limited", status)
		}

		// Wait for event to expire
		time.Sleep(200 * time.Millisecond)

		// Should not be limited after expiration
		status, err = rl.IsLimited("192.0.2.1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("after expiration: IsLimited() = %+v, want not limited", status)
		}
	})

	t.Run("ipv6 address", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			KeyPrefix: "test:",
		})

		// First two requests with IPv6 (should be allowed)
		for i := 0; i < 2; i++ {
			status, err := rl.IsLimited("2001:db8::1", "test-agent")
			if err != nil {
				t.Fatalf("IsLimited() error = %v", err)
			}
			if status.IsLimited {
				t.Errorf("request %d: IsLimited() = %+v, want not limited", i, status)
			}
		}

		// Third request (should be IP-limited)
		status, err := rl.IsLimited("2001:db8::1", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if !status.IsLimited {
			t.Error("IsLimited() = not limited, want limited")
		}

		// Different IPv6 should not be limited
		status, err = rl.IsLimited("2001:db8::2", "test-agent")
		if err != nil {
			t.Fatalf("IsLimited() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("different IPv6: IsLimited() = %+v, want not limited", status)
		}
	})

	t.Run("redis errors", func(t *testing.T) {
		cleanRedis()
		failPool := &redis.Pool{
			MaxIdle:     3,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				conn, err := redis.Dial("tcp", s.Addr())
				if err != nil {
					return nil, err
				}
				conn.Close()
				return conn, nil
			},
		}

		rl := NewRateLimiter(failPool, RateLimitConfig{
			IPConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			IPUAConfig: LimitConfig{
				Interval:  time.Hour,
				MaxEvents: 2,
			},
			KeyPrefix: "test:",
		})

		_, err := rl.IsLimited("192.0.2.1", "test-agent")
		if err == nil {
			t.Error("IsLimited() with closed connection, want error, got nil")
		}
	})
}

func TestRateLimiter_IsLimitedWithTier(t *testing.T) {
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

	t.Run("under tier limit", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig:   LimitConfig{Interval: time.Hour, MaxEvents: 60},
			IPUAConfig: LimitConfig{Interval: time.Hour, MaxEvents: 30},
			KeyPrefix:  "test:",
		})

		tierConfig := LimitConfig{
			Interval:  time.Hour,
			MaxEvents: 100,
		}

		status, err := rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("IsLimitedWithTier() = %+v, want not limited", status)
		}
	})

	t.Run("tier limit exceeded", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig:   LimitConfig{Interval: time.Hour, MaxEvents: 60},
			IPUAConfig: LimitConfig{Interval: time.Hour, MaxEvents: 30},
			KeyPrefix:  "test:",
		})

		tierConfig := LimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
		}

		// First two requests (should be allowed)
		for i := 0; i < 2; i++ {
			status, err := rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
			if err != nil {
				t.Fatalf("IsLimitedWithTier() error = %v", err)
			}
			if status.IsLimited {
				t.Errorf("request %d: IsLimitedWithTier() = %+v, want not limited", i, status)
			}
		}

		// Third request (should be tier-limited)
		status, err := rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if !status.IsLimited {
			t.Error("IsLimitedWithTier() = not limited, want limited")
		}
		if status.LimitType != "tier" {
			t.Errorf("LimitType = %s, want tier", status.LimitType)
		}
	})

	t.Run("different org+ip combinations not limited", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig:   LimitConfig{Interval: time.Hour, MaxEvents: 60},
			IPUAConfig: LimitConfig{Interval: time.Hour, MaxEvents: 30},
			KeyPrefix:  "test:",
		})

		tierConfig := LimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
		}

		// Max out companyX + IP1
		for i := 0; i < 2; i++ {
			_, err := rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
			if err != nil {
				t.Fatalf("IsLimitedWithTier() error = %v", err)
			}
		}

		// Different org, same IP should not be limited
		status, err := rl.IsLimitedWithTier("companyY", "192.0.2.1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("different org: IsLimitedWithTier() = %+v, want not limited", status)
		}

		// Same org, different IP should not be limited
		status, err = rl.IsLimitedWithTier("companyX", "192.0.2.2", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("different IP: IsLimitedWithTier() = %+v, want not limited", status)
		}
	})

	t.Run("tier events expire", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig:   LimitConfig{Interval: time.Hour, MaxEvents: 60},
			IPUAConfig: LimitConfig{Interval: time.Hour, MaxEvents: 30},
			KeyPrefix:  "test:",
		})

		tierConfig := LimitConfig{
			Interval:  100 * time.Millisecond,
			MaxEvents: 2,
		}

		// First event
		status, err := rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("first event: IsLimitedWithTier() = %+v, want not limited", status)
		}

		// Wait for event to expire
		time.Sleep(200 * time.Millisecond)

		// Should not be limited after expiration
		status, err = rl.IsLimitedWithTier("companyX", "192.0.2.1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("after expiration: IsLimitedWithTier() = %+v, want not limited", status)
		}
	})

	t.Run("ipv6 address", func(t *testing.T) {
		cleanRedis()
		rl := NewRateLimiter(pool, RateLimitConfig{
			IPConfig:   LimitConfig{Interval: time.Hour, MaxEvents: 60},
			IPUAConfig: LimitConfig{Interval: time.Hour, MaxEvents: 30},
			KeyPrefix:  "test:",
		})

		tierConfig := LimitConfig{
			Interval:  time.Hour,
			MaxEvents: 2,
		}

		// First two requests with IPv6 (should be allowed)
		for i := 0; i < 2; i++ {
			status, err := rl.IsLimitedWithTier("companyX", "2001:db8::1", tierConfig)
			if err != nil {
				t.Fatalf("IsLimitedWithTier() error = %v", err)
			}
			if status.IsLimited {
				t.Errorf("request %d: IsLimitedWithTier() = %+v, want not limited", i, status)
			}
		}

		// Third request (should be tier-limited)
		status, err := rl.IsLimitedWithTier("companyX", "2001:db8::1", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if !status.IsLimited {
			t.Error("IsLimitedWithTier() = not limited, want limited")
		}

		// Different IPv6 should not be limited
		status, err = rl.IsLimitedWithTier("companyX", "2001:db8::2", tierConfig)
		if err != nil {
			t.Fatalf("IsLimitedWithTier() error = %v", err)
		}
		if status.IsLimited {
			t.Errorf("different IPv6: IsLimitedWithTier() = %+v, want not limited", status)
		}
	})
}
