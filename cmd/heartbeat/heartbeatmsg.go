package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
)

// LoadRegistration downloads the registration data from the registration
// URL and matches it with the provided hostname.
func LoadRegistration(ctx context.Context, hostname string, url *url.URL) (*v2.Registration, error) {
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

	var registrations map[string]v2.Registration
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
