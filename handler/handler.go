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
	"github.com/m-lab/locate/proxy"
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
	targetTmpl *template.Template
}

// Locator defines how the TranslatedQuery handler requests machines nearest to
// the client.
type Locator interface {
	Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error)
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
}

// NewClient creates a new client.
func NewClient(project string, private Signer, locator Locator) *Client {
	return &Client{
		Signer:     private,
		project:    project,
		Locator:    locator,
		targetTmpl: template.Must(template.New("name").Parse("{{.Experiment}}-{{.Machine}}{{.Host}}")),
	}
}

// NewClientDirect creates a new client with a target template using only the target machine.
func NewClientDirect(project string, private Signer, locator Locator) *Client {
	return &Client{
		Signer:  private,
		project: project,
		Locator: locator,
		// Useful for the locatetest package when running a local server.
		targetTmpl: template.Must(template.New("name").Parse("{{.Machine}}{{.Host}}")),
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

var (
	latlonMethod  = "appengine-latlong"
	regionMethod  = "appengine-region"
	countryMethod = "appengine-country"
	noneMethod    = "appengine-none"
	nullLatLon    = "0.000000,0.000000"
)

func findLocation(rw http.ResponseWriter, req *http.Request) (string, string) {
	headers := req.Header
	fields := log.Fields{
		"CityLatLong": headers.Get("X-AppEngine-CityLatLong"),
		"Country":     headers.Get("X-AppEngine-Country"),
		"Region":      headers.Get("X-AppEngine-Region"),
		"Proto":       headers.Get("X-Forwarded-Proto"),
		"Path":        req.URL.Path,
	}

	// First, try the given lat/lon. Avoid invalid values like 0,0.
	latlon := headers.Get("X-AppEngine-CityLatLong")
	if lat, lon := splitLatLon(latlon); latlon != nullLatLon && lat != "" && lon != "" {
		log.WithFields(fields).Info(latlonMethod)
		rw.Header().Set("X-Locate-ClientLatLon", latlon)
		rw.Header().Set("X-Locate-ClientLatLon-Method", latlonMethod)
		return lat, lon
	}
	// The next two fallback methods require the country, so check this next.
	country := headers.Get("X-AppEngine-Country")
	if country == "" || static.Countries[country] == "" {
		// Without a valid country value, we can neither lookup the
		// region nor country.
		log.WithFields(fields).Info(noneMethod)
		rw.Header().Set("X-Locate-ClientLatLon-Method", noneMethod)
		return "", ""
	}
	// Second, country is valid, so try to lookup region.
	region := strings.ToUpper(headers.Get("X-AppEngine-Region"))
	if region != "" && static.Regions[country+"-"+region] != "" {
		latlon = static.Regions[country+"-"+region]
		log.WithFields(fields).Info(regionMethod)
		rw.Header().Set("X-Locate-ClientLatLon", latlon)
		rw.Header().Set("X-Locate-ClientLatLon-Method", regionMethod)
		return splitLatLon(latlon)
	}
	// Third, region was not found, fallback to using the country.
	latlon = static.Countries[country]
	log.WithFields(fields).Info(countryMethod)
	rw.Header().Set("X-Locate-ClientLatLon", latlon)
	rw.Header().Set("X-Locate-ClientLatLon-Method", countryMethod)
	return splitLatLon(latlon)
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

	// Make proxy request using AppEngine provided lat,lon.
	lat, lon := findLocation(rw, req)
	targets, err := c.Nearest(req.Context(), service, lat, lon)
	if err != nil {
		status := http.StatusInternalServerError
		if err == proxy.ErrNoContent {
			status = http.StatusServiceUnavailable
		}
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

// Heartbeat implements /v2/heartbeat requests.
func (c *Client) Heartbeat(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNotImplemented)
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
