package loadbalanced

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/m-lab/go/host"
)

// LBChecker ...
// type LBChecker struct {
// 	project string
// 	host    string
// 	compute *compute.RegionBackendServicesService
// }

// NewLBChecker ...
// func NewLBChecker(project string, host host.Name, computeSvc *compute.Service) *LBChecker {
// 	return &LBChecker{
// 		project: project,
// 		host:    strings.ReplaceAll(host.String(), ".", "-"),
// 		compute: compute.NewRegionBackendServicesService(computeSvc),
// 	}
// }

// GetHealth ...
// func (lbc *LBChecker) GetHealth(ctx context.Context) float64 {
// 	group := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/%s/instanceGroups/%s", lbc.project, "us-west1", lbc.host)
// 	call := lbc.compute.GetHealth(lbc.project, "us-west1", lbc.host, &compute.ResourceGroupReference{
// 		Group: group,
// 	}).Context(ctx)
// 	fmt.Printf("lbc: %+v\n", lbc)
// 	health, err := call.Do()
// 	health.HealthStatus[0]
// 	fmt.Println("error: ", err)
// 	fmt.Printf("result: %+v\n", health)
// 	if err != nil {
// 		return 0
// 	}
// 	return 1
// }

type LBChecker struct {
	client *compute.RegionBackendServicesClient
	bs     *computepb.BackendService
}

func NewLBChecker(ctx context.Context, project string, host host.Name) *LBChecker {
	c, err := compute.NewRegionBackendServicesRESTClient(ctx)
	if err != nil {
		fmt.Println(err)
	}

	req := &computepb.GetRegionBackendServiceRequest{
		BackendService: strings.ReplaceAll(host.String(), ".", "-"),
		Project:        project,
		Region:         "us-west1",
	}
	bs, err := c.Get(ctx, req)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("%+v\n", bs)

	return &LBChecker{
		client: c,
		bs:     bs,
	}
}

func (lbc *LBChecker) GetHealth(ctx context.Context) float64 {
	h, err := lbc.client.GetHealth(ctx, &computepb.GetHealthRegionBackendServiceRequest{})
	if err != nil {
		return 0
	}
	return 1
}
