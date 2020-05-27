// Package proxy issues requests to the legacy mlab-ns service and parses responses.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var (
	// DefaultLegacyServer is the default service for requesting nearest machines.
	DefaultLegacyServer = "https://mlab-ns.appspot.com"
)

// geoOptions and target are types for decoding native mlab-ns responses.
type geoOptions []target

type target struct {
	FQDN    string `json:"fqdn"`
	City    string `json:"city"`
	Country string `json:"country"`
}

// LegacyLocator manages requests to the legacy mlab-ns service.
type LegacyLocator struct {
	Server  url.URL
	project string
}

// MustNewLegacyLocator creates a new LegacyLocator instance. If the given url
// fails to parse, the function will exit.
func MustNewLegacyLocator(u, project string) *LegacyLocator {
	server, err := url.Parse(u)
	rtx.Must(err, "Failed to parse given url: %q", u)
	return &LegacyLocator{
		Server:  *server,
		project: project,
	}
}

// ErrNoContent is returned when mlab-ns returns http.StatusNoContent.
var ErrNoContent = errors.New("no content from server")

// Nearest discovers the nearest machines for the target service, using a
// proxied request to the LegacyServer such as mlab-ns.
func (ll *LegacyLocator) Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error) {
	path, ok := static.LegacyServices[service]
	if !ok {
		return nil, fmt.Errorf("Unsupported service: %q", service)
	}
	params := url.Values{}
	params.Set("policy", "geo_options")
	if lat != "" && lon != "" {
		params.Set("lat", lat)
		params.Set("lon", lon)
	}
	u := ll.Server
	u.Path = path
	u.RawQuery = params.Encode()

	ctxTimeout, ctxCancel := context.WithTimeout(ctx, 20*time.Second)
	defer ctxCancel()
	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	opts := &geoOptions{}
	_, err = UnmarshalResponse(req, opts)
	if err != nil {
		return nil, err
	}
	return ll.collect(opts), nil
}

// UnmarshalResponse reads the response from the given request and unmarshals
// the value into the given result.
func UnmarshalResponse(req *http.Request, result interface{}) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusNoContent {
		// Cannot unmarshal empty content.
		return resp, ErrNoContent
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(b, result)
}

// collect reads all FQDN results from the given options and guarantees to
// return v2 formatted hostnames.
func (ll *LegacyLocator) collect(opts *geoOptions) []v2.Target {
	targets := []v2.Target{}
	for _, opt := range *opts {
		name, err := host.Parse(opt.FQDN)
		if err != nil {
			continue
		}
		target := v2.Target{
			Machine: name.String(),
			Location: &v2.Location{
				City:    opt.City,
				Country: opt.Country,
			},
		}
		// Convert the service name into a canonical machine name.
		targets = append(targets, target)
	}
	return targets
}
