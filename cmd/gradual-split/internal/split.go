package internal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	appengine "google.golang.org/api/appengine/v1"
)

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

func LookupLatestVersion(ctx context.Context, api AppWrapper, verFrom string) (string, error) {
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

func GetVersions(ctx context.Context, api AppWrapper, service *appengine.Service, vfrom, vto string) (string, string, error) {
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
			vto, err = LookupLatestVersion(ctx, api, vfrom)
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

func PerformSplit(ctx context.Context, api AppWrapper, service *appengine.Service, delay time.Duration, vfrom, vto string) error {
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
