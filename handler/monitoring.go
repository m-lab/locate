package handler

import (
	"net/http"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

// Monitoring issues access tokens for end to end monitoring requests.
func (c *Client) Monitoring(rw http.ResponseWriter, req *http.Request) {
	result := v2.MonitoringResult{}

	// Validate request.
	cl := controller.GetClaim(req.Context())
	if cl == nil {
		result.Error = v2.NewError("claim", "Must provide access_token", http.StatusBadRequest)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Check that the given subject appears to be an M-Lab machine name.
	m, err := host.Parse(cl.Subject)
	if err != nil {
		result.Error = v2.NewError("subject", "Subject must be specified", http.StatusBadRequest)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Lookup service configuration.
	experiment, service := getExperimentAndService(req.URL.Path)
	ports, ok := static.Configs[service]
	if !ok {
		result.Error = v2.NewError("config", "Unknown service: "+service, http.StatusBadRequest)
		writeResult(rw, result.Error.Status, &result)
		return
	}

	// Preserve other, given request parameters.
	values := req.URL.Query()
	values.Del("access_token")

	// Get monitoring subject access tokens for the given machine.
	machine := cl.Subject
	token := c.getAccessToken(cl.Subject, static.SubjectMonitoring)
	// NOTE: v2 vs v3 naming
	// v2 monitoring uses the non-service, machine name as the subject.
	// v3 monitoring uses the service name as the subject, so this should be a noop.
	m.Service = experiment
	hostname := m.StringWithService()
	urls := c.getURLs(ports, hostname, token, values)
	result.AccessToken = token
	result.Target = &v2.Target{
		// Monitoring results only include one target.
		Machine:  machine,
		Hostname: hostname,
		URLs:     urls,
	}
	result.Results = append(result.Results, v2.Target{
		Machine:  machine,
		Hostname: hostname,
		URLs:     urls,
	})
	writeResult(rw, http.StatusOK, &result)
}
