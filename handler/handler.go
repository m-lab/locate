// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"

	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/access/token"
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
}

// NewClient creates a new client.
func NewClient(project string, private *token.Signer) *Client {
	return &Client{
		Signer:  private,
		project: project,
	}
}

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
	experiment, datatype := getDatatypeAndExperiment(req.URL.Path)
	service := experiment + "/" + datatype

	// Check whether the service is valid before all other steps to fail fast.
	ports, ok := static.Configs[service]
	if !ok {
		result.Error = v2.NewError("config", "Unknown datatype/service: "+service, http.StatusBadRequest)
		writeResult(rw, &result)
		return
	}

	// Make proxy request using AppEngine provided lat,lon.
	lat, lon := splitLatLon(req.Header.Get("X-AppEngine-CityLatLong"))
	machines, err := proxy.Nearest(req.Context(), service, lat, lon)
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

	// Populate the URLs using the ports configuration.
	for i := range targets {
		urls, err := c.getURLs(ports, targets[i].Machine, experiment)
		if err != nil {
			log.Println(err)
			continue
		}
		targets[i].URLs = urls
	}
	result.Results = targets
	writeResult(rw, &result)
}

// Monitoring implements /v2/monitoring requests.
func (c *Client) Monitoring(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNotImplemented)
}

// Heartbeat implements /v2/heartbeat requests.
func (c *Client) Heartbeat(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNotImplemented)
}

func (c *Client) getURLs(ports static.Ports, machine, service string) (map[string]string, error) {
	urls := map[string]string{}
	for name, target := range ports {
		// TODO: generate real access tokens.
		token := "this-is-a-fake-token"
		target.Host = fmt.Sprintf("%s-%s:%s", service, machine, target.Host)
		target.RawQuery = "access_token=" + token
		urls[name] = target.String()
	}
	return urls, nil
}

func writeResult(rw http.ResponseWriter, result *v2.QueryResult) {
	// Write response.
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	if result.Error != nil {
		rw.WriteHeader(result.Error.Status)
	}
	rw.Write(b)
}

// getDatatypeAndExperiment takes an http request path and extracts the last two
// fields. For correct requests (e.g. "/v2/query/ndt/ndt5"), this will be the
// experiment name (e.g. "ndt") and the datatype (e.g. "ndt5").
func getDatatypeAndExperiment(p string) (string, string) {
	datatype := path.Base(p)
	experiment := path.Base(path.Dir(p))
	return experiment, datatype
}
