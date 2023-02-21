package internal

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	appengine "google.golang.org/api/appengine/v1"
)

// Split represents traffic ratios "split" between two versions.
type Split struct {
	From float64
	To   float64
}

// SplitOptions collects multiple values needed to perform the gradual split.
type SplitOptions struct {
	From     string
	To       string
	Delay    time.Duration
	Sequence []Split
}

// AppWrapper provides fakeable access to the App Engine Admin API.
type AppWrapper interface {
	VersionPages(ctx context.Context, serviceID string, f func(listVer *appengine.ListVersionsResponse) error) error
	ServiceUpdate(ctx context.Context, serviceID string, service *appengine.Service, mask string) (*appengine.Operation, error)
}

// lookupLatestVersion returns the latest serving version string. The return
// value will be empty if verFrom is the latest version.
func lookupLatestVersion(ctx context.Context, api AppWrapper, verFrom string) (string, error) {
	latest := ""
	err := api.VersionPages(ctx, "locate", func(lv *appengine.ListVersionsResponse) error {
		for _, version := range lv.Versions {
			if version.ServingStatus != "SERVING" {
				// We can only split to versions that are running.
				continue
			}
			// Find the latest version.
			if version.Id > latest {
				latest = version.Id
			}
		}
		return nil
	})
	if verFrom >= latest {
		// Skip older versions.
		return "", err
	}
	return latest, err
}

// GetVersions returns the current active and latest version. If vfrom and vto
// are provided, they can override the result.
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

// PerformSplit applies the sequence of split options pausing by Delay after
// each step. PerformSplit can resume a split in progress.
func PerformSplit(ctx context.Context, api AppWrapper, service *appengine.Service, opt *SplitOptions) error {
	// Check which split sequence position to start from. We can assume that
	// vfrom will always be present in the currnet split Allocation.
	for i := range opt.Sequence {
		split := opt.Sequence[i]
		if split.To <= service.Split.Allocations[opt.To] {
			continue
		}
		fmt.Print("Splitting traffic from:", opt.From, split.From, "-> to:", opt.To, split.To, "")
		service.Split.Allocations[opt.From] = split.From
		service.Split.Allocations[opt.To] = split.To
		if split.From == 0.0 {
			// You cannot set a split percentage of zero.
			delete(service.Split.Allocations, opt.From)
		}
		service.Split.ShardBy = "IP" // Make traffic sticky.
		op, err := api.ServiceUpdate(ctx, "locate", service, "split")
		if err != nil {
			return fmt.Errorf("%v: failed to update service traffic split: %#v", err, op)
		}
		fmt.Println("sleeping", opt.Delay)
		time.Sleep(opt.Delay)
	}
	return nil
}
