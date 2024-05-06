package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/host"
	"github.com/m-lab/go/memoryless"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/metrics"
)

// Loader is a structure to load registration data from siteinfo.
type Loader struct {
	Ticker   *memoryless.Ticker // Ticker determines the interval to reload the data.
	url      *url.URL
	hostname host.Name
	exp      string
	svcs     map[string][]string
	reg      v2.Registration
}

// NewLoader returns a new loader for registration data.
func NewLoader(ctx context.Context, url *url.URL, hostname, exp string, svcs map[string][]string, config memoryless.Config) (*Loader, error) {
	h, err := host.Parse(hostname)
	if err != nil {
		return nil, err
	}

	if url == nil {
		return nil, content.ErrUnsupportedURLScheme
	}

	ticker, err := memoryless.NewTicker(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Loader{
		Ticker:   ticker,
		url:      url,
		hostname: h,
		exp:      exp,
		svcs:     svcs,
	}, nil
}

// GetRegistration downloads the registration data from the registration
// URL and matches it with the provided hostname.
func (ldr *Loader) GetRegistration(ctx context.Context) (*v2.Registration, error) {
	provider, err := content.FromURL(ctx, ldr.url)
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

	// The registration key can be both a hostname or a hostname with a service,
	// so the following code checks for both, with priority to hostnames w/o service.
	v, ok := registrations[ldr.hostname.String()]
	if !ok {
		v, ok = registrations[ldr.hostname.StringWithService()]
	}
	if ok {
		v.Hostname = ldr.hostname.StringWithService()
		// If the registration has not changed, there is nothing new to return.
		if cmp.Equal(ldr.reg, v) {
			return nil, nil
		}

		ldr.reg = v
		v.Experiment = ldr.exp
		v.Services = ldr.svcs
		metrics.RegistrationUpdateTime.Set(float64(time.Now().Unix()))
		return &v, nil
	}

	return nil, fmt.Errorf("hostname %s not found", ldr.hostname)
}
