package health

import (
	"context"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	gax "github.com/googleapis/gax-go/v2"
)

// GCPChecker queries the VM's load balancer to check its status.
type GCPChecker struct {
	client GCEClient
	md     Metadata
}

// Metadata returns environmental metadata for a machine.
type Metadata interface {
	Project() string
	Backend() string
	Region() string
	Group() string
}

// GCEClient queries the Compute API for health updates.
type GCEClient interface {
	GetHealth(context.Context, *computepb.GetHealthRegionBackendServiceRequest, ...gax.CallOption) (*computepb.BackendServiceGroupHealth, error)
}

// NewGCPChecker returns a new instance of GCPChecker.
func NewGCPChecker(c GCEClient, md Metadata) *GCPChecker {
	return &GCPChecker{
		client: c,
		md:     md,
	}
}

// GetHealth contacts the GCP load balancer to get the latest VM health status
// and uses the data to generate a health score.
func (c *GCPChecker) GetHealth(ctx context.Context) float64 {
	g := c.md.Group()
	req := &computepb.GetHealthRegionBackendServiceRequest{
		BackendService: c.md.Backend(),
		Project:        c.md.Project(),
		Region:         c.md.Region(),
		ResourceGroupReferenceResource: &computepb.ResourceGroupReference{
			Group: &g,
		},
	}
	lbHealth, err := c.client.GetHealth(ctx, req)
	if err != nil {
		return 0
	}

	for _, h := range lbHealth.HealthStatus {
		// The group is healthy if at least one of the instances has a 'HEALTHY' health state.
		if strings.EqualFold(*h.HealthState, "HEALTHY") {
			return 1
		}
	}

	return 0
}
