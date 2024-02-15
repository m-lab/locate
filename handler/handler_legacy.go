package handler

import (
	"encoding/json"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/rtx"
	v1 "github.com/m-lab/locate/api/v1"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
)

/*
Based on historical use, most requests include no parameters. When requests
included parameters, the following are most frequent.

	no parameters		0.74
	format=json			0.07
	policy=geo_options	0.05
	ip 					0.04
	format=bt 			0.04
	policy=geo 			0.03
	policy=random      >0.01

Options found in historical requests that will not be supported:
  - address_family=ipv4 - not supportable
  - address_family=ipv6 - not supportable
  - policy=metro & metro - too few, site= is still available.
  - longitude, latitude - never supported
  - ip - not right now

Supported options:
  - format=json (default)
  - policy=geo (default) (1 result)
  - policy=geo_options (4 result)
  - format=bt (1 result)
  - policy=random (1 result)
  - lat=xxx&lon=yyy
*/
func (c *Client) LegacyNearest(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	results := v1.Results{}
	setHeaders(rw)

	if c.limitRequest(time.Now().UTC(), req) {
		writeResultLegacy(rw, http.StatusTooManyRequests, &results)
		metrics.RequestsTotal.WithLabelValues("nearest", "request limit", http.StatusText(http.StatusTooManyRequests)).Inc()
		return
	}

	experiment, service := getLegacyExperimentAndService(req.URL.Path)

	// Look up client location.
	loc, err := c.checkClientLocation(rw, req)
	if err != nil {
		status := http.StatusServiceUnavailable
		writeResultLegacy(rw, status, &results)
		metrics.RequestsTotal.WithLabelValues("nearest", "client location", http.StatusText(status)).Inc()
		return
	}

	// Parse client location.
	lat, errLat := strconv.ParseFloat(loc.Latitude, 64)
	lon, errLon := strconv.ParseFloat(loc.Longitude, 64)
	if errLat != nil || errLon != nil {
		status := http.StatusInternalServerError
		writeResultLegacy(rw, status, &results)
		metrics.RequestsTotal.WithLabelValues("nearest", "parse client location", http.StatusText(status)).Inc()
		return
	}

	q := req.URL.Query()
	// Find the nearest targets using the client parameters.
	// Unconditionally, limit to the physical nodes for legacy requests.
	opts := &heartbeat.NearestOptions{Type: "physical"}
	// TODO(soltesz): support 204 if no results found.
	targetInfo, err := c.LocatorV2.Nearest(service, lat, lon, opts)
	if err != nil {
		status := http.StatusInternalServerError
		writeResultLegacy(rw, status, &results)
		metrics.RequestsTotal.WithLabelValues("nearest", "server location", http.StatusText(status)).Inc()
		return
	}

	pOpts := paramOpts{raw: req.Form, version: "v1", ranks: targetInfo.Ranks}
	// Populate target URLs and write out response.
	c.populateURLs(targetInfo.Targets, targetInfo.URLs, experiment, pOpts)
	results = translate(experiment, targetInfo)
	var result interface{}
	// TODO(soltesz): format=bt & format=json
	// TODO(soltesz): policy=geo & policy=geo_options & policy=random
	switch q.Get("policy") {
	case "geo_options":
		result = results
	default:
		result = results[0]
	}
	// Default format is JSON.
	writeResultLegacy(rw, http.StatusOK, result)
	metrics.RequestsTotal.WithLabelValues("nearest", "success", http.StatusText(http.StatusOK)).Inc()
}

func writeResultLegacy(rw http.ResponseWriter, status int, result interface{}) {
	b, err := json.Marshal(result)
	// Errors are only possible when marshalling incompatible types, like functions.
	rtx.PanicOnError(err, "Failed to format result")
	rw.WriteHeader(status)
	rw.Write(b)
}

func translate(experiment string, info *heartbeat.TargetInfo) v1.Results {
	results := v1.Results{}
	for i := range info.Targets {
		h, err := host.Parse(info.Targets[i].Machine)
		if err != nil {
			continue
		}
		results = append(results, v1.Result{
			City:    info.Targets[i].Location.City,
			Country: info.Targets[i].Location.Country,
			Site:    h.Site,
			FQDN:    experiment + "-" + info.Targets[i].Machine,
		})
	}

	return results
}

func getLegacyExperimentAndService(p string) (string, string) {
	service, ok := static.LegacyConvert[p]
	if !ok {
		return "", ""
	}
	datatype := path.Base(service)
	experiment := path.Base(path.Dir(service))
	return experiment, experiment + "/" + datatype
}
