package limits

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

// getRedisMemory returns the current memory usage in bytes
func getRedisMemory(pool *redis.Pool) (int64, error) {
	conn := pool.Get()
	defer conn.Close()

	info, err := redis.String(conn.Do("INFO", "memory"))
	if err != nil {
		return 0, err
	}

	var used int64
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "used_memory:") {
			fmt.Sscanf(line, "used_memory:%d", &used)
			break
		}
	}
	return used, nil
}

func BenchmarkRateLimiter_RealWorld(b *testing.B) {
	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", "localhost:6379")
		},
	}

	// Create rate limiter with 60 requests/hour limit
	limiter := NewRateLimiter(pool, RateLimitConfig{
		Interval:  time.Hour,
		MaxEvents: 60,
		KeyPrefix: "benchmark:",
	})

	// Clean up before and after benchmark
	conn := pool.Get()
	conn.Do("FLUSHDB")
	conn.Close()

	defer func() {
		conn := pool.Get()
		defer conn.Close()
		conn.Do("FLUSHDB")
	}()

	tests := []struct {
		name     string
		duration time.Duration
		ipRange  int // Number of unique IPs
		uaRange  int // Number of unique UAs
	}{
		{
			name:     "SingleIPUA_Long",
			duration: 5 * time.Second,
			ipRange:  1,
			uaRange:  1,
		},
		{
			name:     "ManyIPs_OneUA",
			duration: 5 * time.Second,
			ipRange:  100000,
			uaRange:  1,
		},
		{
			name:     "OneIP_ManyUAs",
			duration: 5 * time.Second,
			ipRange:  1,
			uaRange:  100000,
		},
		{
			name:     "ManyIPs_ManyUAs",
			duration: 5 * time.Second,
			ipRange:  100,
			uaRange:  10000,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Get initial memory usage
			memBefore, err := getRedisMemory(pool)
			if err != nil {
				b.Fatalf("Failed to get initial memory: %v", err)
			}

			// Run benchmark with custom timer
			b.ResetTimer()
			start := time.Now()
			ops := 0
			for time.Since(start) < tt.duration {
				ip := fmt.Sprintf("%d", ops%tt.ipRange)
				ua := fmt.Sprintf("%d", ops%tt.uaRange)

				_, err := limiter.IsLimited(ip, ua)
				if err != nil {
					b.Fatalf("IsLimited failed: %v", err)
				}
				ops++
			}
			b.StopTimer()

			// Get final memory usage.
			memAfter, err := getRedisMemory(pool)
			if err != nil {
				b.Fatalf("Failed to get final memory: %v", err)
			}

			// Add custom metrics.
			duration := time.Since(start)
			opsPerSec := float64(ops) / duration.Seconds()
			memoryUsed := memAfter - memBefore

			b.ReportMetric(float64(memoryUsed), "memory_bytes")
			b.ReportMetric(opsPerSec, "ops/sec")
			b.ReportMetric(float64(memoryUsed)/float64(ops), "bytes/key")

			b.Logf("Total operations: %d", ops)
			b.Logf("Memory used: %.2f MB", float64(memoryUsed)/(1024*1024))
			b.Logf("Memory per key: %.2f bytes", float64(memoryUsed)/float64(ops))
			b.Logf("Operations/sec: %.2f", opsPerSec)
		})

		// Clean up between tests
		conn := pool.Get()
		//conn.Do("FLUSHDB")
		conn.Close()
	}
}
