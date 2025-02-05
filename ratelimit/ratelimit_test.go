package ratelimit

import (
	"context"
	"errors"
	"testing"
)

// mockSketch implements sketch.Sketch for testing
type mockSketch struct {
	count    int64
	incErr   error
	countErr error
}

func (m *mockSketch) Increment(ctx context.Context, item string) error {
	if m.incErr != nil {
		return m.incErr
	}
	return nil
}

func (m *mockSketch) Count(ctx context.Context, item string) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.count, nil
}

func (m *mockSketch) Reset(ctx context.Context) error {
	m.count = 0
	return nil
}

func TestLimiter_Allow(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		sketch    *mockSketch
		wantAllow bool
		wantErr   bool
	}{
		{
			name:      "under-limit",
			config:    Config{RequestsPerMinute: 10},
			sketch:    &mockSketch{count: 5},
			wantAllow: true,
			wantErr:   false,
		},
		{
			name:      "at-limit",
			config:    Config{RequestsPerMinute: 10},
			sketch:    &mockSketch{count: 10},
			wantAllow: true,
			wantErr:   false,
		},
		{
			name:      "over-limit",
			config:    Config{RequestsPerMinute: 10},
			sketch:    &mockSketch{count: 11},
			wantAllow: false,
			wantErr:   false,
		},
		{
			name:      "increment-error",
			config:    Config{RequestsPerMinute: 10},
			sketch:    &mockSketch{incErr: errors.New("redis down")},
			wantAllow: true, // fail open
			wantErr:   true,
		},
		{
			name:      "count-error",
			config:    Config{RequestsPerMinute: 10},
			sketch:    &mockSketch{countErr: errors.New("redis down")},
			wantAllow: true, // fail open
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New(tt.config, tt.sketch)
			got, err := l.Allow(context.Background(), "test-ip")

			if (err != nil) != tt.wantErr {
				t.Errorf("Allow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.wantAllow {
				t.Errorf("Allow() = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}
