package locatetest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/m-lab/go/testingx"
	"github.com/m-lab/locate/api/locate"
	"github.com/m-lab/locate/api/v2"
)

// defines expectations for returned v2.Target URLs
type v2TargetURLExpectation struct {
	key         string
	valuePrefix string
}

// defines what we expect v2.Target to be
type v2TargetExpectation struct {
	machine  string
	hostname string
	loc      *v2.Location
	urls     []v2TargetURLExpectation
}

func (exp *v2TargetExpectation) check(t *testing.T, target *v2.Target) {
	// Machine
	if target.Machine != exp.machine {
		t.Error("expected", exp.machine, "got", target.Machine)
	}

	// Hostname
	if target.Hostname != exp.hostname {
		t.Error("expected", exp.hostname, "got", target.Hostname)
	}

	// Location
	switch {
	case exp.loc == nil && target.Location == nil:
		// nothing

	case exp.loc != nil && target.Location == nil:
	case exp.loc == nil && target.Location != nil:
		t.Errorf("expected %+v, got %+v", exp.loc, target.Location)

	case exp.loc.City != target.Location.City:
		t.Error("expected", exp.loc.City, target.Location.City)
		fallthrough
	case exp.loc.Country != target.Location.Country:
		t.Error("expected", exp.loc.Country, target.Location.Country)
	}

	// URLs
	if len(exp.urls) != len(target.URLs) {
		t.Fatal("expected", len(exp.urls), "got", len(target.URLs))
	}
	for idx := 0; idx < len(target.URLs); idx++ {
		expect := exp.urls[0]
		got, ok := target.URLs[expect.key]
		if !ok {
			t.Error("missing entry for", expect.key)
			continue
		}
		samePrefix := strings.HasPrefix(got, expect.valuePrefix)
		if !samePrefix {
			t.Error(got, "does not start with", expect.valuePrefix)
		}
	}
}

// callback for TestLocateServer ensures the result is correct.
func testDefaultNdt7Targets(machine string) func(t *testing.T, targets []v2.Target) {
	return func(t *testing.T, targets []v2.Target) {
		// Ensure the stack trace does not include this helper
		t.Helper()

		// Prepare the expectations
		expect := &v2TargetExpectation{
			machine:  machine,
			hostname: machine,
			loc: &v2.Location{
				City:    "Ventimiglia",
				Country: "IT",
			},
			urls: []v2TargetURLExpectation{
				{
					key:         "ws:///ndt/v7/download",
					valuePrefix: fmt.Sprintf("ws://%s/ndt/v7/download", machine),
				},
				{
					key:         "ws:///ndt/v7/upload",
					valuePrefix: fmt.Sprintf("ws://%s/ndt/v7/upload", machine),
				},
				{
					key:         "wss:///ndt/v7/download",
					valuePrefix: fmt.Sprintf("wss://%s/ndt/v7/download", machine),
				},
				{
					key:         "wss:///ndt/v7/upload",
					valuePrefix: fmt.Sprintf("wss://%s/ndt/v7/upload", machine),
				},
			},
		}

		// We already checked the count is 1 so we feel entitled
		// to just grabbing the first and only entry
		target0 := targets[0]

		// Compare with the expectations
		expect.check(t, &target0)
	}
}

func TestLocateServer(t *testing.T) {

	// define all the test cases
	tests := []struct {
		// name is the name of the testcase
		name string

		// locator is the [*LocatorV2] to use
		locator *LocatorV2

		// path is the URL path to use.
		path string

		// expectErr indicates whether we expect an error
		expectErr bool

		// expectCount is the number of expected entries
		expectCount int

		// deepCheck is an optional function for deeply checking
		// whether the returned targets are OK.
		deepCheck func(t *testing.T, targets []v2.Target)
	}{

		{
			name: "success-locate-server-v2-with-domain",
			locator: &LocatorV2{
				TargetInfo: NewTargetInfoNdt7("mlab1-mil07.mlab-oti.measurement-lab.org"),
			},
			path:        "/v2/nearest",
			expectCount: 1,
			deepCheck:   testDefaultNdt7Targets("mlab1-mil07.mlab-oti.measurement-lab.org"),
		},

		{
			name: "success-locate-server-v2-with-IP-address",
			locator: &LocatorV2{
				TargetInfo: NewTargetInfoNdt7("130.192.91.211:54321"),
			},
			path:        "/v2/nearest",
			expectCount: 1,
			deepCheck:   testDefaultNdt7Targets("130.192.91.211:54321"),
		},

		{
			name: "error-locate-server-v2",
			locator: &LocatorV2{
				Err: errors.New("fake error"),
			},
			path:      "/v2/nearest",
			expectErr: true,
		},

		{
			name:      "neither-error-not-target-info",
			locator:   &LocatorV2{ /* empty */ },
			path:      "/v2/nearest",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			srv := NewLocateServerV2(tt.locator)
			t.Cleanup(srv.Close)

			c := locate.NewClient("fake-user-agent")
			u, err := url.Parse(srv.URL)
			testingx.Must(t, err, "failed to parse locatetest url")
			u.Path = tt.path
			c.BaseURL = u

			// NOTE: only known services (e.g. ndt/ndt7) are supported by the locate API.
			targets, err := c.Nearest(ctx, "ndt/ndt7")

			if tt.expectErr != (err != nil) {
				t.Errorf("NewLocateServer() = expectErr %v, got %v", tt.expectErr, err)
			}

			if tt.expectCount != len(targets) {
				t.Errorf("NewLocateServer() = expectCount %d, got %d", tt.expectCount, len(targets))
			}

			if tt.deepCheck != nil {
				tt.deepCheck(t, targets)
			}
		})
	}
}
