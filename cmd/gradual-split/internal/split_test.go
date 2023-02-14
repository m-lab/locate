package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	appengine "google.golang.org/api/appengine/v1"
)

type fakeAPI struct {
	versions   []*appengine.Version
	versionErr error
	serviceErr error
}

func (a *fakeAPI) VersionPages(ctx context.Context, serviceID string,
	f func(listVer *appengine.ListVersionsResponse) error) error {
	lvs := &appengine.ListVersionsResponse{
		Versions: a.versions,
	}
	if a.versionErr != nil {
		return a.versionErr
	}
	return f(lvs)
}

func (a *fakeAPI) ServiceUpdate(ctx context.Context, serviceID string, service *appengine.Service, mask string) (*appengine.Operation, error) {
	if a.serviceErr != nil {
		return nil, a.serviceErr
	}
	return nil, nil // a.apis.Apps.Services.Patch(project, serviceID, service).UpdateMask(mask).Do()
}

func Test_GetVersions(t *testing.T) {
	tests := []struct {
		name        string
		versions    []*appengine.Version
		allocations map[string]float64
		from        string
		to          string
		wantFrom    string
		wantTo      string
		versionErr  error
		wantErr     bool
	}{
		{
			name: "success",
			versions: []*appengine.Version{
				&appengine.Version{ServingStatus: "SERVING", Id: "a"},
				&appengine.Version{ServingStatus: "STOPPED", Id: "b"},
				&appengine.Version{ServingStatus: "SERVING", Id: "c"},
			},
			allocations: map[string]float64{
				"a": 1.0,
			},
			wantFrom: "a",
			wantTo:   "c",
		},
		{
			name: "success",
			allocations: map[string]float64{
				"a": 1.0,
			},
			from:     "a",
			to:       "c",
			wantFrom: "a",
			wantTo:   "c",
		},
		{
			name: "success",
			versions: []*appengine.Version{
				&appengine.Version{ServingStatus: "SERVING", Id: "a"},
			},
			allocations: map[string]float64{
				"a": 1.0,
			},
			wantFrom: "a",
			wantTo:   "a",
		},
		{
			name: "success",
			allocations: map[string]float64{
				"a": 0.5,
				"b": 0.5,
			},
			wantFrom: "a",
			wantTo:   "b",
		},
		{
			name: "error",
			allocations: map[string]float64{
				"a": 0.2,
				"b": 0.3,
				"c": 0.5,
			},
			wantErr: true,
		},
		{
			name: "error",
			versions: []*appengine.Version{
				&appengine.Version{ServingStatus: "SERVING", Id: "a"},
			},
			allocations: map[string]float64{
				"a": 1.0,
			},
			versionErr: errors.New("fake version error"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			api := &fakeAPI{versions: tt.versions, versionErr: tt.versionErr}
			service := &appengine.Service{
				Split: &appengine.TrafficSplit{
					Allocations: tt.allocations,
				},
			}
			gotFrom, gotTo, err := GetVersions(ctx, api, service, tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFrom != tt.wantFrom {
				t.Errorf("GetVersions() got = %v, want %v", gotFrom, tt.wantFrom)
			}
			if gotTo != tt.wantTo {
				t.Errorf("GetVersions() got1 = %v, want %v", gotTo, tt.wantTo)
			}
		})
	}
}

func Test_PerformSplit(t *testing.T) {
	tests := []struct {
		name       string
		vfrom      string
		vto        string
		serviceErr error
		wantErr    bool
	}{
		{
			name:  "success",
			vfrom: "a",
			vto:   "c",
		},
		{
			name:       "error",
			serviceErr: errors.New("fake service error"),
			wantErr:    true,
		},
	}
	delay := time.Millisecond
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			api := &fakeAPI{
				versions: []*appengine.Version{
					&appengine.Version{ServingStatus: "SERVING", Id: "a"},
					&appengine.Version{ServingStatus: "SERVING", Id: "c"},
				},
				serviceErr: tt.serviceErr,
			}
			service := &appengine.Service{
				Split: &appengine.TrafficSplit{
					Allocations: map[string]float64{
						"a": 0.5,
						"c": 0.5,
					},
				},
			}
			opt := &SplitOptions{
				From:  tt.vfrom,
				To:    tt.vto,
				Delay: delay,
				Sequence: []Split{
					{From: 0.90, To: 0.10}, // the biggest disruption appears to happen in the first step.
					{From: 0.75, To: 0.25},
					{From: 0.50, To: 0.50},
					{From: 0.25, To: 0.75},
					{From: 0.00, To: 1.00},
				},
			}

			err := PerformSplit(ctx, api, service, opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("PerformSplit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
