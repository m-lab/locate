package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/host"
)

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

	var registrations []RegistrationMessage
	err = json.Unmarshal(exp, &registrations)
	if err != nil {
		return nil, err
	}

	for _, r := range registrations {
		if r.Site == h.Site && r.Machine == h.Machine {
			r.Hostname = hostname
			// TODO(cristinaleon): populate experiment and services.
			return &r, nil
		}
	}

	return nil, fmt.Errorf("hostname %s not found", hostname)
}
