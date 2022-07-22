// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
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
	ClientLocator
	StatusTracker
	targetTmpl *template.Template
}

// Locator defines how the TranslatedQuery handler requests machines nearest to
// the client.
type Locator interface {
	Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error)
}

// ClientLocator defines the interfeace for looking up the client geo location.
type ClientLocator interface {
	Locate(req *http.Request) (*clientgeo.Location, error)
}

// StatusTracker defines the interface for tracking the status of experiment instances.
type StatusTracker interface {
	RegisterInstance(rm v2.Registration) error
	UpdateHealth(hostname string, hm v2.Health) error
	Instances() map[string]v2.HeartbeatMessage
	StopImport()
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
}

// NewClient creates a new client.
func NewClient(project string, private Signer, locator Locator, client ClientLocator, tracker StatusTracker) *Client {
	return &Client{
		Signer:        private,
		project:       project,
		Locator:       locator,
		ClientLocator: client,
		StatusTracker: tracker,
		targetTmpl:    template.Must(template.New("name").Parse("{{.Experiment}}-{{.Machine}}{{.Host}}")),
	}
}

// NewClientDirect creates a new client with a target template using only the target machine.
func NewClientDirect(project string, private Signer, locator Locator, client ClientLocator, tracker StatusTracker) *Client {
	return &Client{
		Signer:        private,
		project:       project,
		Locator:       locator,
		ClientLocator: client,
		StatusTracker: tracker,
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

	// Set CORS policy to allow third-party websites to use returned resources.
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	// Prevent caching of result.
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
	rw.Header().Set("Cache-Control", "no-store")

	// Check whether the service is valid before all other steps to fail fast.
	ports, ok := static.Configs[service]
	if !ok {
		result.Error = v2.NewError("config", "Unknown service: "+service, http.StatusBadRequest)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Lookup the client location using the client request.
	loc, err := c.Locate(req)
	if err != nil {
		status := http.StatusServiceUnavailable
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", status)
		writeResult(rw, result.Error.Status, &result)
		return
	}
	// Copy location headers to response writer.
	for key := range loc.Headers {
		rw.Header().Set(key, loc.Headers.Get(key))
	}
	// Find the nearest targets using provided lat,lon.
	targets, err := c.Nearest(req.Context(), service, loc.Latitude, loc.Longitude)
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

	// Populate each set of URLs using the ports configuration.
	for i := range targets {
		token := c.getAccessToken(targets[i].Machine, experiment)
		targets[i].URLs = c.getURLs(ports, targets[i].Machine, experiment, token, clientValues(req.Form))
	}
	result.Results = targets
	writeResult(rw, http.StatusOK, &result)
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
