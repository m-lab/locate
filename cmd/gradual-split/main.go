package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

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

type split struct {
	From float64
	To   float64
}

var splitSequence = []split{
	{0.9, 0.1}, // the biggest disruption appears to happen in the first step.
	{0.75, 0.25},
	{0.5, 0.5},
	{0.25, 0.75},
	{0.0, 1},
}

type AppWrapper interface {
	VersionPages(ctx context.Context, serviceID string, f func(listVer *appengine.ListVersionsResponse) error) error
	ServiceUpdate(ctx context.Context, serviceID string, service *appengine.Service, mask string) (*appengine.Operation, error)
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

func lookupLatestVersion(ctx context.Context, api AppWrapper, verFrom string) (string, error) {
	latest := ""
	err := api.VersionPages(ctx, "locate", func(lv *appengine.ListVersionsResponse) error {
		for _, version := range lv.Versions {
			if version.ServingStatus != "SERVING" {
				// We can only split to versions that are running.
				continue
			}
			if verFrom >= version.Id {
				// Skip older versions.
				continue
			}
			// Find the latest version.
			if version.Id > latest {
				latest = version.Id
			}
		}
		return nil
	})
	return latest, err
}

func getVersions(ctx context.Context, api AppWrapper, service *appengine.Service, vfrom, vto string) (string, string, error) {
	switch {
	case vfrom != "" && vto != "":
		// Assume the source and target versions have been provided on the command line.
		break
	case len(service.Split.Allocations) == 1:
		// Assume the split has not started.
		for from := range service.Split.Allocations {
			vfrom = from
			break
		}
		if vto == "" {
			var err error
			vto, err = lookupLatestVersion(ctx, api, vfrom)
			if err != nil {
				return "", "", err
			}
		}
		if vto == "" {
			// There is no later version.
			return vfrom, vfrom, nil
		}
	case len(service.Split.Allocations) == 2:
		// Assume the split is already in progress.
		versions := []string{}
		for from := range service.Split.Allocations {
			versions = append(versions, from)
		}
		sort.Strings(versions)
		vfrom, vto = versions[0], versions[1]
	default:
		return "", "", errors.New("multi split not supported")
	}
	return vfrom, vto, nil
}

func performSplit(ctx context.Context, api AppWrapper, service *appengine.Service, vfrom, vto string) error {
	// Check which split sequence position to start from. We can assume that
	// vfrom will always be present in the currnet split Allocation.
	for i := range splitSequence {
		split := splitSequence[i]
		if split.To <= service.Split.Allocations[vto] {
			continue
		}
		fmt.Println("Splitting traffic from:", vfrom, split.From, "-> to:", vto, split.To)
		service.Split.Allocations[vfrom] = split.From
		service.Split.Allocations[vto] = split.To
		if split.From == 0.0 {
			// You cannot set a split percentage of zero.
			delete(service.Split.Allocations, vfrom)
		}
		service.Split.ShardBy = "IP" // Make traffic sticky.
		op, err := api.ServiceUpdate(ctx, "locate", service, "split")
		if err != nil {
			return fmt.Errorf("%v: failed to update service traffic split: %#v", err, op)
		}
		fmt.Println("Sleeping", delay)
		time.Sleep(delay)
	}
	return nil
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

	vfrom, vto, err := getVersions(ctx, api, service, versionFrom, versionTo)
	rtx.Must(err, "failed to get versions")
	if vfrom == vto {
		fmt.Println("Traffic already split to latest version")
		return
	}

	fmt.Println("Starting split:", vfrom, vto, service.Split.Allocations)
	err = performSplit(ctx, api, service, vfrom, vto)
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
