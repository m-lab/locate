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
	"regexp"
	"time"

	"github.com/m-lab/locate/static"
)

var (
	// LegacyServer controls the target service for requesting nearest machines.
	LegacyServer = url.URL{Scheme: "https", Host: "mlab-ns.appspot.com"}
)

// geoOptions and target are types for decoding native mlab-ns responses.
type geoOptions []target

type target struct {
	FQDN string `json:"fqdn"`
}

// ErrNoContent is returned when mlab-ns returns http.StatusNoContent.
var ErrNoContent = errors.New("no content from server")

// Nearest discovers the nearest machines for the target service, using a
// proxied request to the LegacyServer such as mlab-ns.
func Nearest(ctx context.Context, service, lat, lon string) ([]string, error) {
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
	u := LegacyServer
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

// NOTE: this pattern assumes mlab-ns is returning flat host names, and should
// be future compatible with project-decorated DNS names.
var machinePattern = regexp.MustCompile("([a-z-]+)-(mlab[1-4][.-][a-z]{3}[0-9ct]{2}(\\.mlab-[a-z]+)?.measurement-lab.org)")

func collect(opts *geoOptions) []string {
	targets := []string{}
	for _, opt := range *opts {
		fields := machinePattern.FindStringSubmatch(opt.FQDN)
		if len(fields) < 3 {
			continue
		}
		targets = append(targets, fields[2])
	}
	return targets
}
