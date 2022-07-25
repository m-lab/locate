// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/heartbeat"
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
	LocatorV2
	ClientLocator
	targetTmpl *template.Template
}

// Locator defines how the TranslatedQuery handler requests machines nearest to
// the client.
type Locator interface {
	Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error)
}

// LocatorV2 ...
type LocatorV2 interface {
	Nearest(service, typ string, lat, lon float64) ([]v2.Target, []url.URL, error)
	heartbeat.StatusTracker
}

// ClientLocator defines the interfeace for looking up the client geo location.
type ClientLocator interface {
	Locate(req *http.Request) (*clientgeo.Location, error)
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
}

// NewClient creates a new client.
func NewClient(project string, private Signer, locator Locator, locatorV2 LocatorV2, client ClientLocator) *Client {
	return &Client{
		Signer:        private,
		project:       project,
		Locator:       locator,
		LocatorV2:     locatorV2,
		ClientLocator: client,
		targetTmpl:    template.Must(template.New("name").Parse("{{.Experiment}}-{{.Machine}}{{.Host}}")),
	}
}

// NewClientDirect creates a new client with a target template using only the target machine.
func NewClientDirect(project string, private Signer, locator Locator, locatorV2 LocatorV2, client ClientLocator) *Client {
	return &Client{
		Signer:        private,
		project:       project,
		Locator:       locator,
		LocatorV2:     locatorV2,
		ClientLocator: client,
		// Useful for the locatetest package when running a local server.
		targetTmpl: template.Must(template.New("name").Parse("{{.Machine}}{{.Host}}")),
	}
}

func clientValues(raw url.Values) url.Values {
	v := url.Values{}
	for key := range raw {
		if strings.HasPrefix(key, "client_") {
			// note: we only use the first value.
			v.Set(key, raw.Get(key))
		}
	}
	return v
}

// TranslatedQuery uses the legacy mlab-ns service for liveness as a
// transitional step in loading state directly.
func (c *Client) TranslatedQuery(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm() // Parse any raw query parameters into req.Form url.Values.
	result := v2.NearestResult{}
	experiment, service := getExperimentAndService(req.URL.Path)
	setHeaders(rw)

	// Check whether the service is valid before all other steps to fail fast.
	ports, ok := static.Configs[service]
	if !ok {
		result.Error = v2.NewError("config", "Unknown service: "+service, http.StatusBadRequest)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Look up client location.
	loc, err := c.checkClientLocation(rw, req)
	if err != nil {
		return
	}

	// Find the nearest targets using provided lat,lon.
	targets, err := c.Locator.Nearest(req.Context(), service, loc.Latitude, loc.Longitude)
	if err != nil {
		status := http.StatusInternalServerError
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", status)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Update targets with empty URLs.
	for i := range targets {
		targets[i].URLs = map[string]string{}
	}

	// Populate targets URLs and write out response.
	c.populateURLs(targets, ports, experiment, req.Form)
	result.Results = targets
	writeResult(rw, http.StatusOK, &result)
}

// Nearest implements /v2beta/nearest requests and uses the LocatorV2 to lookup
// nearest servers.
func (c *Client) Nearest(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	result := v2.NearestResult{}
	experiment, service := getExperimentAndService(req.URL.Path)
	setHeaders(rw)

	// Look up client location.
	loc, err := c.checkClientLocation(rw, req)
	if err != nil {
		return
	}

	// Parse client location.
	lat, errLat := strconv.ParseFloat(loc.Latitude, 64)
	lon, errLon := strconv.ParseFloat(loc.Longitude, 64)
	if errLat != nil || errLon != nil {
		result.Error = v2.NewError("client", "Failed to parse client location", http.StatusInternalServerError)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Find the nearest targets using the client parameters.
	t := req.URL.Query().Get("type")
	targets, urls, err := c.LocatorV2.Nearest(service, t, lat, lon)
	if err != nil {
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", http.StatusInternalServerError)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Populate target URLs and write out response.
	c.populateURLs(targets, urls, experiment, req.Form)
	result.Results = targets
	writeResult(rw, http.StatusOK, &result)
}

// checkClientLocation looks up the client location and copies the location
// headers to the response writer.
func (c *Client) checkClientLocation(rw http.ResponseWriter, req *http.Request) (*clientgeo.Location, error) {
	// Lookup the client location using the client request.
	loc, err := c.Locate(req)
	if err != nil {
		status := http.StatusServiceUnavailable
		result := v2.NearestResult{
			Error: v2.NewError("client", "Failed to lookup client location", status),
		}
		writeResult(rw, result.Error.Status, &result)
		return nil, fmt.Errorf("Failed to lookup client location")
	}

	// Copy location headers to response writer.
	for key := range loc.Headers {
		rw.Header().Set(key, loc.Headers.Get(key))
	}

	return loc, nil
}

// populateURLs populates each set of URLs using the target configuration.
func (c *Client) populateURLs(targets []v2.Target, ports static.Ports, exp string, form url.Values) {
	for i := range targets {
		token := c.getAccessToken(targets[i].Machine, exp)
		targets[i].URLs = c.getURLs(ports, targets[i].Machine, exp, token, clientValues(form))
	}
}

// getAccessToken allocates a new access token using the given machine name as
// the intended audience and the subject as the target service.
func (c *Client) getAccessToken(machine, subject string) string {
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
	return token
}

// getURLs creates URLs for the named experiment, running on the named machine
// for each given port. Every URL will include an `access_token=` parameter,
// authorizing the measurement.
func (c *Client) getURLs(ports static.Ports, machine, experiment, token string, extra url.Values) map[string]string {
	urls := map[string]string{}
	// For each port config, prepare the target url with access_token and
	// complete host field.
	for _, target := range ports {
		name := target.String()
		params := url.Values{}
		params.Set("access_token", token)
		for key := range extra {
			// note: we only use the first value.
			params.Set(key, extra.Get(key))
		}
		target.RawQuery = params.Encode()

		host := &bytes.Buffer{}
		err := c.targetTmpl.Execute(host, map[string]string{
			"Experiment": experiment,
			"Machine":    machine,
			"Host":       target.Host, // from URL template, so typically just the ":port".
		})
		rtx.PanicOnError(err, "bad template evaluation")
		target.Host = host.String()
		urls[name] = target.String()
	}
	return urls
}

// setHeaders sets the response headers for "nearest" requests.
func setHeaders(rw http.ResponseWriter) {
	// Set CORS policy to allow third-party websites to use returned resources.
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	// Prevent caching of result.
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
	rw.Header().Set("Cache-Control", "no-store")
}

// writeResult marshals the result and writes the result to the response writer.
func writeResult(rw http.ResponseWriter, status int, result interface{}) {
	b, err := json.MarshalIndent(result, "", "  ")
	// Errors are only possible when marshalling incompatible types, like functions.
	rtx.PanicOnError(err, "Failed to format result")
	rw.WriteHeader(status)
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
