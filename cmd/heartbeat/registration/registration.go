package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/host"
)

// RegistrationMessage contains a set of identifying fields
// for a server instance.
type RegistrationMessage struct {
	City          string
	CountryCode   string
	ContinentCode string
	Experiment    string
	Hostname      string
	Latitude      float64
	Longitude     float64
	Machine       string
	Metro         string
	Project       string
	Site          string
	Type          string
	Uplink        string
	Services      map[string][]string
}

// Load downloads the registration data from the registration
// URL and matches it with the provided hostname.
func Load(ctx context.Context, hostname string, url *url.URL) (*RegistrationMessage, error) {
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

	var registrations map[string]RegistrationMessage
	err = json.Unmarshal(exp, &registrations)
	if err != nil {
		return nil, err
	}

	if v, ok := registrations[h.String()]; ok {
		v.Hostname = hostname
		return &v, nil
	}

	return nil, fmt.Errorf("hostname %s not found", hostname)
}
