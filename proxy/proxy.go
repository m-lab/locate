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
	"github.com/m-lab/locate/static"
)

var (
	// DefaultLegacyServer is the default service for requesting nearest machines.
	DefaultLegacyServer = "https://mlab-ns.appspot.com"
)

// geoOptions and target are types for decoding native mlab-ns responses.
type geoOptions []target

type target struct {
	FQDN string `json:"fqdn"`
}

// LegacyLocator manages requests to the legacy mlab-ns service.
type LegacyLocator struct {
	Server url.URL
}

// MustNewLegacyLocator creates a new LegacyLocator instance. If the given url
// fails to parse, the function will exit.
func MustNewLegacyLocator(u string) *LegacyLocator {
	server, err := url.Parse(u)
	rtx.Must(err, "Failed to parse given url: %q", u)
	return &LegacyLocator{
		Server: *server,
	}
}

// ErrNoContent is returned when mlab-ns returns http.StatusNoContent.
var ErrNoContent = errors.New("no content from server")

// Nearest discovers the nearest machines for the target service, using a
// proxied request to the LegacyServer such as mlab-ns.
func (ll *LegacyLocator) Nearest(ctx context.Context, service, lat, lon string) ([]string, error) {
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
	err = UnmarshalResponse(req, opts)
	if err != nil {
		return nil, err
	}
	return collect(opts), nil
}

// UnmarshalResponse reads the response from the given request and unmarshals
// the value into the given result.
func UnmarshalResponse(req *http.Request, result interface{}) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNoContent {
		// Cannot unmarshal empty content.
		return ErrNoContent
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, result)
}

func collect(opts *geoOptions) []string {
	targets := []string{}
	for _, opt := range *opts {
		name, err := host.Parse(opt.FQDN)
		if err != nil {
			continue
		}
		// Convert the service name into a canonical machine name.
		targets = append(targets, name.String())
	}
	return targets
}
