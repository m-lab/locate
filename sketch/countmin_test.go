package sketch

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"
)

func setupTestRedis(t *testing.T) (*redis.Pool, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	pool := &redis.Pool{
		MaxIdle:     1,
		IdleTimeout: 10 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", mr.Addr())
		},
	}

	return pool, func() {
		pool.Close()
		mr.Close()
	}
}

func TestCMSketch_SingleItem(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := Config{
		Width:          1000,
		Depth:          3,
		Window:         time.Minute,
		RedisKeyPrefix: "test",
	}

	sketch := New(config, client)

	// Test single increment and count
	err := sketch.Increment(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to increment: %v", err)
	}

	count, err := sketch.Count(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestCMSketch_MultipleIncrements(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	config := Config{
		Width:          1000,
		Depth:          3,
		Window:         time.Minute,
		RedisKeyPrefix: "test",
	}

	sketch := New(config, client)

	// Increment same item multiple times
	expectedCount := 5
	for i := 0; i < expectedCount; i++ {
		err := sketch.Increment(ctx, "item1")
		if err != nil {
			t.Fatalf("Failed to increment on iteration %d: %v", i, err)
		}
	}

	// Verify count
	count, err := sketch.Count(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, count)
	}
}

func TestCMSketch_TimeWindow(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	baseTime := time.Now().UTC()
	config := Config{
		Width:          1000,
		Depth:          3,
		Window:         time.Minute,
		RedisKeyPrefix: "test",
	}

	sketch := New(config, client)

	// Increment in current window
	err := sketch.Increment(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to increment: %v", err)
	}

	// Verify count in current window
	count, err := sketch.Count(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected count 1 in current window, got %d", count)
	}

	// Move to next time window
	sketch.nowFunc = func() time.Time {
		return baseTime.Add(time.Minute)
	}

	// Count should be 0 in new window
	count, err = sketch.Count(ctx, "item1")
	if err != nil {
		t.Fatalf("Failed to count in second window: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 in new window, got %d", count)
	}
}

func TestCMSketch_RedisErrors(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestRedis(t)
	defer cleanup()

	config := Config{
		Width:          1000,
		Depth:          3,
		Window:         time.Minute,
		RedisKeyPrefix: "test",
	}

	sketch := New(config, pool)

	// Force Redis connection failure by closing the pool.
	cleanup()

	// Test Increment with broken Redis
	err := sketch.Increment(ctx, "item1")
	if err == nil {
		t.Error("Expected error on Increment with broken Redis connection, got nil")
	}

	// Test Count with broken Redis
	_, err = sketch.Count(ctx, "item1")
	if err == nil {
		t.Error("Expected error on Count with broken Redis connection, got nil")
	}
}
