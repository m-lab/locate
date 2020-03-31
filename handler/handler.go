// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"

	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/access/token"
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
}

// NewClient creates a new client.
func NewClient(project string, private *token.Signer) *Client {
	return &Client{
		Signer:  private,
		project: project,
	}
}

// TranslatedQuery uses the legacy mlab-ns service for liveness as a
// transitional step in loading state directly.
func (c *Client) TranslatedQuery(rw http.ResponseWriter, req *http.Request) {
	// TODO: make request to mlab-ns and translate reply to v2.QueryReply.
	result := v2.QueryResult{}
	service, datatype := getDatatypeAndService(req.URL.Path)
	key := service + "/" + datatype

	ports, ok := static.Configs[key]
	if !ok {
		result.Error = v2.NewError("config", "Unknown datatype/service: "+key, http.StatusBadRequest)
		writeResult(rw, &result)
		return
	}

	// TODO: read real targets from mlab-ns reply.
	targets := []v2.Target{
		{Machine: "mlab1-lga0t", URLs: map[string]string{}},
	}

	for i := range targets {
		urls, err := c.getURLs(ports, targets[i].Machine, service)
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
		target.Host = fmt.Sprintf("%s-%s.%s.measurement-lab.org:%s", service, machine, c.project, target.Host)
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

// getDatatypeAndService takes an http request path and extracts the last two
// fields. For correct requests (e.g. "/v2/query/ndt/ndt5"), this will be the
// service name (e.g. "ndt") and the datatype (e.g. "ndt5").
func getDatatypeAndService(p string) (string, string) {
	datatype := path.Base(p)
	service := path.Base(path.Dir(p))
	return service, datatype
}
