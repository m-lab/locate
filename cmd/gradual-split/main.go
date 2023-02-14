package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/m-lab/locate/cmd/gradual-split/internal"

	"golang.org/x/oauth2/google"
	appengine "google.golang.org/api/appengine/v1"
	"google.golang.org/api/option"

	"github.com/m-lab/go/rtx"
)

var (
	project     string
	versionFrom string
	versionTo   string
	delay       time.Duration
)

func init() {
	flag.StringVar(&project, "project", "", "act on services in given project name")
	flag.StringVar(&versionFrom, "version-from", "", "reduce traffic from this version")
	flag.StringVar(&versionTo, "version-to", "", "increase traffic to this version")
	flag.DurationVar(&delay, "delay", 3*time.Minute, "time to wait between traffic split updates")
	log.SetFlags(log.Lshortfile | log.LstdFlags)
}

// AppAPI implements the AppAPI interface.
type AppAPI struct {
	project string
	apis    *appengine.APIService
}

// NewAppAPI creates a new instance of the AppAPI for the given project.
func NewAppAPI(project string, apis *appengine.APIService) *AppAPI {
	return &AppAPI{project: project, apis: apis}
}

// VersionPages lists all AppEngine versions for the given service and calls
// the given function for each "page" of results.
func (a *AppAPI) VersionPages(ctx context.Context, serviceID string,
	f func(listVer *appengine.ListVersionsResponse) error) error {
	return a.apis.Apps.Services.Versions.List(a.project, serviceID).Pages(ctx, f)
}

func (a *AppAPI) ServiceUpdate(ctx context.Context, serviceID string, service *appengine.Service, mask string) (*appengine.Operation, error) {
	return a.apis.Apps.Services.Patch(project, serviceID, service).UpdateMask(mask).Do()
}

func main() {
	flag.Parse()

	// Create a new authenticated HTTP client.
	ctx := context.Background()
	defaultScopes := []string{appengine.CloudPlatformScope, appengine.AppengineAdminScope}
	client, err := google.DefaultClient(ctx, defaultScopes...)
	rtx.Must(err, "failed to get default google client")

	// Create a new AppEngine service instance.
	apis, err := appengine.NewService(ctx, option.WithHTTPClient(client))
	rtx.Must(err, "failed to create appengine client")

	api := NewAppAPI(project, apis)
	service, err := apis.Apps.Services.Get(project, "locate").Context(ctx).Do()
	rtx.Must(err, "failed to get locate service")

	vfrom, vto, err := internal.GetVersions(ctx, api, service, versionFrom, versionTo)
	rtx.Must(err, "failed to get versions")
	if vfrom == vto {
		fmt.Println("Traffic already split to latest version")
		return
	}
	opt := &internal.SplitOptions{
		From:  vfrom,
		To:    vto,
		Delay: delay,
		Sequence: []internal.Split{
			{From: 0.90, To: 0.10}, // the biggest disruption appears to happen in the first step.
			{From: 0.75, To: 0.25},
			{From: 0.50, To: 0.50},
			{From: 0.25, To: 0.75},
			{From: 0.00, To: 1.00},
		},
	}
	fmt.Println("Starting split:", vfrom, vto, service.Split.Allocations)
	err = internal.PerformSplit(ctx, api, service, opt)
	rtx.Must(err, "failed to perform split")

	// Stop serving the vfrom verison.
	v, err := apis.Apps.Services.Versions.Get(project, "locate", vfrom).Context(ctx).Do()
	rtx.Must(err, "failed to get version to stop serving")

	v.ServingStatus = "STOPPED"
	op, err := apis.Apps.Services.Versions.Patch(
		project, "locate", v.Id, v).UpdateMask("servingStatus").Context(ctx).Do()
	rtx.Must(err, "failed to update version: %#v", op)
	fmt.Println("Completed split:", vto)
}
