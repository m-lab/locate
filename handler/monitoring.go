package handler

import (
	"net/http"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

// Monitoring implements /v2/monitoring requests.
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
	_, err := host.Parse(cl.Subject)
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

	// Get monitoring subject access tokens for the given machine.
	machine := cl.Subject
	token := c.getAccessToken(machine, static.SubjectMonitoring)
	urls := c.getURLs(ports, machine, experiment, token, nil)
	result.AccessToken = token
	result.Target = &v2.Target{
		// Monitoring results only include one target.
		Machine: machine,
		URLs:    urls,
	}
	result.Results = append(result.Results, v2.Target{
		Machine: machine,
		URLs:    urls,
	})
	writeResult(rw, http.StatusOK, &result)
}
