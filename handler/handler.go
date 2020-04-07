// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

// Signer defines how access tokens are signed.
type Signer interface {
	Sign(cl jwt.Claims) (string, error)
}

// Client contains state needed for xyz.
type Client struct {
	Signer
	project string
	Locator
}

// Locator defines how the TranslatedQuery handler requests machines nearest to
// the client.
type Locator interface {
	Nearest(ctx context.Context, service, lat, lon string) ([]string, error)
}

// NewClient creates a new client.
func NewClient(project string, private Signer, locator Locator) *Client {
	return &Client{
		Signer:  private,
		project: project,
		Locator: locator,
	}
}

// splitLatLon attempts to split the "<lat>,<lon>" string provided by AppEngine
// into two fields. The return values preserve the original lat,lon order.
func splitLatLon(latlon string) (string, string) {
	if fields := strings.Split(latlon, ","); len(fields) == 2 {
		return fields[0], fields[1]
	}
	return "", ""
}

// TranslatedQuery uses the legacy mlab-ns service for liveness as a
// transitional step in loading state directly.
func (c *Client) TranslatedQuery(rw http.ResponseWriter, req *http.Request) {
	result := v2.QueryResult{}
	experiment, service := getExperimentAndService(req.URL.Path)

	// Check whether the service is valid before all other steps to fail fast.
	ports, ok := static.Configs[service]
	if !ok {
		result.Error = v2.NewError("config", "Unknown service: "+service, http.StatusBadRequest)
		writeResult(rw, &result)
		return
	}

	// Make proxy request using AppEngine provided lat,lon.
	lat, lon := splitLatLon(req.Header.Get("X-AppEngine-CityLatLong"))
	machines, err := c.Nearest(req.Context(), service, lat, lon)
	if err != nil {
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", http.StatusInternalServerError)
		writeResult(rw, &result)
		return
	}

	// Construct result targets with empty URLs.
	targets := []v2.Target{}
	for i := range machines {
		targets = append(targets, v2.Target{Machine: machines[i], URLs: map[string]string{}})
	}

	// Populate each set of URLs using the ports configuration.
	for i := range targets {
		targets[i].URLs = c.getURLs(ports, targets[i].Machine, experiment, experiment)
	}
	result.Results = targets
	writeResult(rw, &result)
}

// Heartbeat implements /v2/heartbeat requests.
func (c *Client) Heartbeat(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNotImplemented)
}

// getURLs creates URLs for the named experiment, running on the named machine
// for each given port. Every URL will include an `access_token=` parameter,
// authorizing the measurement.
func (c *Client) getURLs(ports static.Ports, machine, experiment, subject string) map[string]string {
	urls := map[string]string{}

	// Create the token. The same access token is used for each target port.
	cl := jwt.Claims{
		Issuer:   static.IssuerLocate,
		Subject:  subject,
		Audience: jwt.Audience{machine},
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	token, err := c.Sign(cl)
	// Sign errors can only happen due to a misconfiguration of the key.
	// A good config will remain good.
	rtx.PanicOnError(err, "signing claims has failed")

	// For each port config, prepare the target url with access_token and
	// complete host field.
	for name, target := range ports {
		params := url.Values{}
		params.Set("access_token", token)
		target.RawQuery = params.Encode()
		target.Host = fmt.Sprintf("%s-%s:%s", experiment, machine, target.Host)
		urls[name] = target.String()
	}
	return urls
}

// writeResult marshals the result and writes the result to the response writer.
func writeResult(rw http.ResponseWriter, result *v2.QueryResult) {
	b, err := json.MarshalIndent(result, "", "  ")
	// Errors are only possible when marshalling incompatible types, like functions.
	rtx.PanicOnError(err, "Failed to format result")
	if result.Error != nil {
		rw.WriteHeader(result.Error.Status)
	}
	rw.Write(b)
}

// getExperimentAndService takes an http request path and extracts the last two
// fields. For correct requests (e.g. "/v2/query/ndt/ndt5"), this will be the
// experiment name (e.g. "ndt") and the datatype (e.g. "ndt5").
func getExperimentAndService(p string) (string, string) {
	datatype := path.Base(p)
	experiment := path.Base(path.Dir(p))
	return experiment, experiment + "/" + datatype
}
