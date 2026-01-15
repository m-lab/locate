package locatetest

import (
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/heartbeat"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

// Signer implements the Signer interface for unit tests.
type Signer struct{}

// Sign creates a fake signature using the given claims.
func (s *Signer) Sign(cl jwt.Claims) (string, error) {
	t := strings.Join([]string{
		cl.Audience[0], cl.Subject, cl.Issuer, cl.Expiry.Time().Format(time.RFC3339),
	}, "--")
	return t, nil
}

// LocatorV2 is a fake LocatorV2 implementation that returns the configured TargetInfo or Err.
type LocatorV2 struct {
	// StatusTracker is here just to satisfy the interface. You don't need to
	// initialize this for typical usage of this code in testing.
	heartbeat.StatusTracker

	// TargetInfo is the optional TargetInfo to return.
	//
	// If both TargetInfo and Err are nil, Nearest returns an error.
	//
	// If both are non-nil, Err takes precedence.
	//
	// The [NewTargetInfoNdt7] func allows to construct a suitable instance that
	// makes sense for returning meaningful ndt7 entries.
	TargetInfo *heartbeat.TargetInfo

	// Err is the optional error to return.
	//
	// If both TargetInfo and Err are nil, Nearest returns an error.
	//
	// If both are non-nil, Err takes precedence.
	Err error
}

// Nearest returns the pre-configured LocatorV2 Servers or Err.
func (l *LocatorV2) Nearest(service string, lat, lon float64, opts *heartbeat.NearestOptions) (*heartbeat.TargetInfo, error) {
	// Design choice: here we *could* do the right thing(tm) in terms of
	// returning the correct structure for each selected service, however,
	// it is more scalable that tests for a specific package do that.
	switch {
	case l.Err != nil:
		return nil, l.Err

	case l.TargetInfo != nil:
		return l.TargetInfo, nil

	default:
		return nil, errors.New("locatorV2: expected non-nil TargetInfo or Err")
	}
}

// NewLocateServerV2 creates an httptest.Server that can respond to Locate API V2
// requests using a LocatorV2. Uselful for unit testing.
func NewLocateServerV2(loc *LocatorV2) *httptest.Server {
	// fake signer, fake locator.
	s := &Signer{}
	c := handler.NewClientDirect("fake-project", s, loc, &clientgeo.NullLocator{}, prom.NewAPI(nil))

	// USER APIs
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/nearest/", http.HandlerFunc(c.Nearest))

	srv := httptest.NewServer(mux)
	log.Println("Listening for INSECURE access requests on " + srv.URL)
	return srv
}

// NewTargetInfoNdt7 is a convenience function to build a [*heartbeat.TargetInfo] to
// initialize a [*LocatorV2] type with targets meaningful for ndt7.
//
// This function servers as a reference implementation example regarding how to fill
// a [*heartbeat.TestInfo] structure for testing. If you have specific needs for other
// experiments hosted by M-Lab, you're probably better off adding the code you need
// inside your implementation rather than inside this package.
//
// The returned response is not exactly the same as a response returned by locate
// because we do not prefix the machine with `ndt-`, which allows to build correct
// URLs regardless of whether machine is an FQDN, an IP address, or an endpoint
// containing an IP address _and_ a port (which is the most common use case in testing).
func NewTargetInfoNdt7(machines ...string) *heartbeat.TargetInfo {

	// Build the targets first. The location does not seem so important and I assume someone
	// who wants a more precise mock could just build the desired struct manually.
	targets := make([]v2.Target, 0, len(machines))
	for _, machine := range machines {
		targets = append(targets, v2.Target{
			Machine:  machine,
			Hostname: machine,
			Location: &v2.Location{
				City:    "Ventimiglia",
				Country: "IT",
			},
		})
	}

	// Assemble the result along with the standard ndt7 schemes.
	return &heartbeat.TargetInfo{
		Targets: targets,
		URLs: []url.URL{
			{Scheme: "ws", Host: "", Path: "/ndt/v7/download"},
			{Scheme: "ws", Host: "", Path: "/ndt/v7/upload"},
			{Scheme: "wss", Host: "", Path: "/ndt/v7/download"},
			{Scheme: "wss", Host: "", Path: "/ndt/v7/upload"},
		},
	}
}
