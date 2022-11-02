package locatetest

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"

	v2 "github.com/m-lab/locate/api/v2"
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

// Locator is a fake Locator interface that returns the configured Servers or Err.
type Locator struct {
	Servers []string
	Err     error
}

// Nearest returns the pre-configured Locator Servers or Err.
func (l *Locator) Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error) {
	if l.Err != nil {
		return nil, l.Err
	}
	t := make([]v2.Target, len(l.Servers))
	for i := range l.Servers {
		t[i].Machine = l.Servers[i]
	}
	return t, nil
}

// LocatorV2 is a fake LocatorV2 implementation that returns the configured Servers or Err.
type LocatorV2 struct {
	heartbeat.StatusTracker
	Servers []string
	Err     error
}

// Nearest returns the pre-configured LocatorV2 Servers or Err.
func (l *LocatorV2) Nearest(service, typ string, lat, lon float64) ([]v2.Target, []url.URL, error) {
	if l.Err != nil {
		return nil, nil, l.Err
	}
	t := make([]v2.Target, len(l.Servers))
	for i := range l.Servers {
		t[i].Machine = l.Servers[i]
	}
	return t, []url.URL{}, nil
}

// NewLocateServer creates an httptest.Server that can respond to Locate API v2
// requests. Useful for unit testing.
func NewLocateServer(loc *Locator) *httptest.Server {
	// fake signer, fake locator.
	s := &Signer{}
	c := handler.NewClientDirect("fake-project", s, loc, &LocatorV2{}, &clientgeo.NullLocator{}, prom.NewAPI(nil))

	// USER APIs
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/nearest/", http.HandlerFunc(c.TranslatedQuery))

	srv := httptest.NewServer(mux)
	log.Println("Listening for INSECURE access requests on " + srv.URL)
	return srv
}

// NewLocateServerV2 creates an httptest.Server that can respond to Locate API V2
// requests using a LocatorV2. Uselful for unit testing.
// TODO(cristinaleon): once the LocatorV2 replaces the Locator, leave only one
// implementation.
func NewLocateServerV2(loc *LocatorV2) *httptest.Server {
	// fake signer, fake locator.
	s := &Signer{}
	c := handler.NewClientDirect("fake-project", s, &Locator{}, loc, &clientgeo.NullLocator{}, prom.NewAPI(nil))

	// USER APIs
	mux := http.NewServeMux()
	mux.HandleFunc("/v2beta2/nearest/", http.HandlerFunc(c.Nearest))

	srv := httptest.NewServer(mux)
	log.Println("Listening for INSECURE access requests on " + srv.URL)
	return srv
}
