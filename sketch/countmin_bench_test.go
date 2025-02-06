package sketch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
)

// setupBenchmarkRedis creates a Redis pool for benchmarking
func setupBenchmarkRedis(b *testing.B) (*redis.Pool, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		b.Fatal(err)
	}

	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	return pool, func() {
		pool.Close()
		mr.Close()
	}
}

func BenchmarkCMSketch_Increment(b *testing.B) {
	ctx := context.Background()
	pool, cleanup := setupBenchmarkRedis(b)
	defer cleanup()

	config := Config{
		Width:          200000,
		Depth:          8,
		Window:         time.Minute,
		RedisKeyPrefix: "bench",
	}

	sketch := New(config, pool)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := sketch.Increment(ctx, "test-ip"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCMSketch_Count(b *testing.B) {
	ctx := context.Background()
	pool, cleanup := setupBenchmarkRedis(b)
	defer cleanup()

	config := Config{
		Width:          200000,
		Depth:          8,
		Window:         time.Minute,
		RedisKeyPrefix: "bench",
	}

	sketch := New(config, pool)

	// First increment a value so we have something to count
	if err := sketch.Increment(ctx, "test-ip"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sketch.Count(ctx, "test-ip"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCMSketch_Sizes tests different sketch sizes
func BenchmarkCMSketch_Sizes(b *testing.B) {
	sizes := []struct {
		width int
		depth int
	}{
		{10000, 4},  // Small
		{50000, 6},  // Medium
		{200000, 8}, // Large
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("width=%d,depth=%d", size.width, size.depth), func(b *testing.B) {
			ctx := context.Background()
			pool, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          size.width,
				Depth:          size.depth,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, pool)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sketch.Increment(ctx, "test-ip"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCMSketch_IPPatterns tests different IP access patterns
func BenchmarkCMSketch_IPPatterns(b *testing.B) {
	patterns := []struct {
		name  string
		getIP func(i int) string
	}{
		{
			name: "single_ip",
			getIP: func(i int) string {
				return "192.168.1.1"
			},
		},
		{
			name: "sequential_ips",
			getIP: func(i int) string {
				return fmt.Sprintf("192.168.1.%d", i%255+1)
			},
		},
		{
			name: "random_ips",
			getIP: func(i int) string {
				return fmt.Sprintf("192.168.%d.%d", (i/255)%255+1, i%255+1)
			},
		},
	}

	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			ctx := context.Background()
			pool, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          200000,
				Depth:          8,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, pool)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sketch.Increment(ctx, p.getIP(i)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCMSketch_Concurrent tests concurrent operations
func BenchmarkCMSketch_Concurrent(b *testing.B) {
	concurrencyLevels := []int{1, 4, 8, 16, 32, 64}

	for _, numGoroutines := range concurrencyLevels {
		b.Run(fmt.Sprintf("goroutines=%d", numGoroutines), func(b *testing.B) {
			ctx := context.Background()
			pool, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          200000,
				Depth:          8,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, pool)

			done := make(chan error)
			opsPerGoroutine := b.N / numGoroutines

			b.ResetTimer()

			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					var err error
					for i := 0; i < opsPerGoroutine; i++ {
						ip := fmt.Sprintf("192.168.%d.%d", goroutineID%255+1, i%255+1)
						if err = sketch.Increment(ctx, ip); err != nil {
							break
						}
						if _, err = sketch.Count(ctx, ip); err != nil {
							break
						}
					}
					done <- err
				}(g)
			}

			for g := 0; g < numGoroutines; g++ {
				if err := <-done; err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
