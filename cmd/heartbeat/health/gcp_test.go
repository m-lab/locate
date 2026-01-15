package health

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
	gax "github.com/googleapis/gax-go/v2"
	"github.com/m-lab/locate/cmd/heartbeat/metadata"
)

func TestGCPChecker_GetHealth(t *testing.T) {
	tests := []struct {
		name   string
		client GCEClient
		want   float64
	}{
		{
			name: "healthy",
			client: &fakeGCEClient{
				status: []string{"HEALTHY"},
				err:    false,
			},
			want: 1,
		},
		{
			name: "unhealthy",
			client: &fakeGCEClient{
				status: []string{"UNHEALTHY"},
				err:    false,
			},
			want: 0,
		},
		{
			name: "mix",
			client: &fakeGCEClient{
				status: []string{"HEALTHY", "HEALTHY", "UNHEALTHY"},
				err:    false,
			},
			want: 1,
		},
		{
			name: "healthy-lower-case",
			client: &fakeGCEClient{
				status: []string{"healthy"},
				err:    false,
			},
			want: 1,
		},
		{
			name: "error",
			client: &fakeGCEClient{
				err: true,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewGCPChecker(tt.client, &metadata.GCPMetadata{})
			if got := c.GetHealth(context.Background()); got != tt.want {
				t.Errorf("GCPChecker.GetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeGCEClient struct {
	status []string
	err    bool
}

func (c *fakeGCEClient) GetHealth(ctx context.Context, req *computepb.GetHealthRegionBackendServiceRequest, opts ...gax.CallOption) (*computepb.BackendServiceGroupHealth, error) {
	if c.err {
		return nil, errors.New("health error")
	}

	health := make([]*computepb.HealthStatus, 0)
	for _, s := range c.status {
		statusPtr := s
		health = append(health, &computepb.HealthStatus{HealthState: &statusPtr})
	}
	return &computepb.BackendServiceGroupHealth{
		HealthStatus: health,
	}, nil
}
