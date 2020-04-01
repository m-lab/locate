// Package proxy issues requests to the legacy mlab-ns service and parses responses.
package proxy

import (
	"context"
	"encoding/json"
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
	LegacyServer = "mlab-ns.appspot.com"
)

type geoOptions []target

type target struct {
	FQDN string `json:"fqdn"`
}

// Nearest discovers the nearest machines for the target service, using a
// proxied request to the LegacyServer such as mlab-ns.
func Nearest(ctx context.Context, service, lat, lon string) ([]string, error) {
	path, ok := static.LegacyServices[service]
	if !ok {
		return nil, fmt.Errorf("Unsupported service: %q", service)
	}
	latlon := ""
	if lat != "" && lon != "" {
		latlon = "&lat=" + lat + "&lon=" + lon
	}
	u := url.URL{
		Scheme:   "https",
		Host:     LegacyServer,
		Path:     path,
		RawQuery: "policy=geo_options" + latlon,
	}
	ctxTimeout, ctxCancel := context.WithTimeout(ctx, 20*time.Second)
	defer ctxCancel()
	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	opts := &geoOptions{}
	err = json.Unmarshal(b, opts)
	if err != nil {
		return nil, err
	}
	return convert(opts), nil
}

// NOTE: this pattern assumes mlab-ns is returning flat host names, and should
// be future compatible with project-decorated DNS names.
var machinePattern = regexp.MustCompile("([a-z-]+)-(mlab[1-4][.-][a-z]{3}[0-9ct]{2}(\\.mlab-[a-z]+)?.measurement-lab.org)")

func convert(opts *geoOptions) []string {
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
