package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/host"
)

// Registration contains a set of identifying fields
// for a server instance.
type Registration struct {
	City          string              // City (e.g., New York).
	CountryCode   string              // Country code (e.g., US).
	ContinentCode string              // Continent code (e.g., NA).
	Experiment    string              // Experiment (e.g., ndt).
	Hostname      string              // Fully qualified service hostname.
	Latitude      float64             // Latitude.
	Longitude     float64             // Longitude.
	Machine       string              // Machine (e.g., mlab1).
	Metro         string              // Metro (e.g., lga).
	Project       string              // Project (e.g., mlab-sandbox).
	Site          string              // Site (e.g.. lga01).
	Type          string              // Machine type (e.g., physical, virtual).
	Uplink        string              // Uplink capacity.
	Services      map[string][]string // Mapping of service names.
}

// Health is the structure used by the heartbeat service
// to report health updates.
type Health struct {
	Hostname string  // Fully qualified service hostname.
	Score    float64 // Health score.
}

// LoadRegistration downloads the registration data from the registration
// URL and matches it with the provided hostname.
func LoadRegistration(ctx context.Context, hostname string, url *url.URL) (*Registration, error) {
	h, err := host.Parse(hostname)
	if err != nil {
		return nil, err
	}

	if url == nil {
		return nil, content.ErrUnsupportedURLScheme
	}

	provider, err := content.FromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	exp, err := provider.Get(ctx)
	if err != nil {
		return nil, err
	}

	var registrations map[string]Registration
	err = json.Unmarshal(exp, &registrations)
	if err != nil {
		return nil, err
	}

	if v, ok := registrations[h.String()]; ok {
		v.Hostname = hostname
		// TODO(cristinaleon): Populate services.
		return &v, nil
	}

	return nil, fmt.Errorf("hostname %s not found", hostname)
}
