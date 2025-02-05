package sketch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var (
	// Can be overridden with flags
	redisAddr = "localhost:6379"
	useReal   = true
)

// setupTestRedis creates either a miniredis or real Redis client based on flags
func setupBenchmarkRedis(b *testing.B) (*redis.Client, func()) {
	if !useReal {
		// Use existing miniredis setup
		mr, err := miniredis.Run()
		if err != nil {
			b.Fatal(err)
		}

		client := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		return client, func() {
			client.Close()
			mr.Close()
		}
	}

	// Use real Redis
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		b.Fatalf("Could not connect to Redis at %s: %v", redisAddr, err)
	}

	return client, func() {
		// Clear test keys before closing
		ctx := context.Background()
		iter := client.Scan(ctx, 0, "bench:*", 0).Iterator()
		for iter.Next(ctx) {
			client.Del(ctx, iter.Val())
		}
		client.Close()
	}
}

func BenchmarkCMSketch_Increment(b *testing.B) {
	ctx := context.Background()
	client, cleanup := setupBenchmarkRedis(b)
	defer cleanup()

	config := Config{
		Width:          200000,
		Depth:          8,
		Window:         time.Minute,
		RedisKeyPrefix: "bench",
	}

	sketch := New(config, client)

	b.ResetTimer() // Don't count setup time
	for i := 0; i < b.N; i++ {
		if err := sketch.Increment(ctx, "test-ip"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCMSketch_Count(b *testing.B) {
	ctx := context.Background()
	client, cleanup := setupBenchmarkRedis(b)
	defer cleanup()

	config := Config{
		Width:          200000,
		Depth:          8,
		Window:         time.Minute,
		RedisKeyPrefix: "bench",
	}

	sketch := New(config, client)

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
		{200000, 8}, // Large (our chosen size)
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("width=%d,depth=%d", size.width, size.depth), func(b *testing.B) {
			ctx := context.Background()
			client, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          size.width,
				Depth:          size.depth,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, client)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sketch.Increment(ctx, "test-ip"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

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
			client, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          200000,
				Depth:          8,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, client)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := sketch.Increment(ctx, p.getIP(i)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCMSketch_RateLimiting simulates actual rate limiting usage
func BenchmarkCMSketch_RateLimiting(b *testing.B) {
	ctx := context.Background()
	client, cleanup := setupBenchmarkRedis(b)
	defer cleanup()

	config := Config{
		Width:          200000,
		Depth:          8,
		Window:         time.Minute,
		RedisKeyPrefix: "bench",
	}

	sketch := New(config, client)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i%255+1)

		// This is what the rate limiter actually does
		if err := sketch.Increment(ctx, ip); err != nil {
			b.Fatal(err)
		}
		if _, err := sketch.Count(ctx, ip); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCMSketch_Concurrent(b *testing.B) {
	// Test different concurrency levels
	concurrencyLevels := []int{1, 4, 8, 16, 32, 64}

	for _, numGoroutines := range concurrencyLevels {
		b.Run(fmt.Sprintf("goroutines=%d", numGoroutines), func(b *testing.B) {
			ctx := context.Background()
			client, cleanup := setupBenchmarkRedis(b)
			defer cleanup()

			config := Config{
				Width:          200000,
				Depth:          8,
				Window:         time.Minute,
				RedisKeyPrefix: "bench",
			}

			sketch := New(config, client)

			// Create a channel to coordinate goroutines
			done := make(chan error)

			b.ResetTimer()

			// Each goroutine will do b.N/numGoroutines operations
			opsPerGoroutine := b.N / numGoroutines

			for g := 0; g < numGoroutines; g++ {
				go func(goroutineID int) {
					var err error
					for i := 0; i < opsPerGoroutine; i++ {
						// Use different IPs for different goroutines
						ip := fmt.Sprintf("192.168.%d.%d", goroutineID%255+1, i%255+1)

						// Simulate complete rate limiting operation
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

			// Wait for all goroutines to complete
			for g := 0; g < numGoroutines; g++ {
				if err := <-done; err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
