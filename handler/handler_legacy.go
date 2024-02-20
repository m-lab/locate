package handler

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

// LegacyNearest is provided for backward compatibility until
// users can migrate to supported, v2 resources.
//
// Based on historical use, most requests include no parameters. When requests
// included parameters, the following are most frequent.
//   - no parameters                    0.813
//   - format=json                      0.068
//   - format=bt&ip=&policy=geo_options 0.059
//   - format=json&policy=geo           0.022
//   - policy=geo                       0.012
//   - policy=geo_options               0.011
//   - policy=random                    0.003
//   - address_family=ipv4&format=json  0.002
//   - address_family=ipv6&format=json  0.002
//   - format=json&metro=&policy=metro  0.001
//
// Options found in historical requests that will not be supported:
//   - address_family=ipv4 - not supportable
//   - address_family=ipv6 - not supportable
//   - policy=metro & metro - too few and site= is still available.
//   - longitude, latitude - never supported
//   - ip - available via user location
//
// Supported options:
//   - format=json        (default)
//   - format=bt
//   - policy=geo         (1 result) (default)
//   - policy=geo_options (4 result)
//   - policy=random      (1 result)
//   - lat=xxx&lon=yyy    available via user location
func (c *Client) LegacyNearest(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	setHeaders(rw)

	// Honor limits for requests.
	if c.limitRequest(time.Now().UTC(), req) {
		rw.WriteHeader(http.StatusTooManyRequests)
		metrics.RequestsTotal.WithLabelValues("legacy", "request limit", http.StatusText(http.StatusTooManyRequests)).Inc()
		return
	}

	// Check that the requested path is for a known service.
	experiment, service := getLegacyExperimentAndService(req.URL.Path)
	if experiment == "" || service == "" {
		rw.WriteHeader(http.StatusBadRequest)
		metrics.RequestsTotal.WithLabelValues("legacy", "bad request", http.StatusText(http.StatusBadRequest)).Inc()
		return
	}

	var lat, lon float64
	q := req.URL.Query()
	if q.Get("policy") == "random" {
		// Generate a random lat/lon for server search.
		lat = (rand.Float64() - 0.5) * 180 // [-90 to 90)
		lon = (rand.Float64() - 0.5) * 360 // [-180 to 180)
	} else {
		// Look up client location.
		loc, err := c.checkClientLocation(rw, req)
		if err != nil {
			status := http.StatusServiceUnavailable
			rw.WriteHeader(status)
			metrics.RequestsTotal.WithLabelValues("legacy", "client location", http.StatusText(status)).Inc()
			return
		}
		// Parse client location.
		var errLat, errLon error
		lat, errLat = strconv.ParseFloat(loc.Latitude, 64)
		lon, errLon = strconv.ParseFloat(loc.Longitude, 64)
		if errLat != nil || errLon != nil {
			status := http.StatusInternalServerError
			rw.WriteHeader(status)
			metrics.RequestsTotal.WithLabelValues("legacy", "parse client location", http.StatusText(status)).Inc()
			return
		}
	}

	// Find the nearest targets using the client parameters.
	// Unconditionally, limit to the physical nodes for legacy requests.
	opts := &heartbeat.NearestOptions{Type: "physical"}
	// TODO(soltesz): support 204 if no results found.
	targetInfo, err := c.LocatorV2.Nearest(service, lat, lon, opts)
	if err != nil {
		status := http.StatusInternalServerError
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("legacy", "server location", http.StatusText(status)).Inc()
		return
	}

	pOpts := paramOpts{raw: req.Form, version: "v1", ranks: targetInfo.Ranks}
	// Populate target URLs and write out response.
	c.populateURLs(targetInfo.Targets, targetInfo.URLs, experiment, pOpts)
	results := translate(experiment, targetInfo)
	// Default policy is a single result.
	switch q.Get("policy") {
	case "geo_options":
		// all results
		break
	default:
		results = results[:1]
	}

	// Default format is JSON.
	switch q.Get("format") {
	case "bt":
		writeBTLegacy(rw, http.StatusOK, results)
	default:
		writeJSONLegacy(rw, http.StatusOK, results)
	}
	metrics.RequestsTotal.WithLabelValues("legacy", "success", http.StatusText(http.StatusOK)).Inc()
}

// writeBTLegacy supports a format used by the uTorrent client, one of the earliest integrations with NDT.
func writeBTLegacy(rw http.ResponseWriter, status int, results v1.Results) {
	rw.WriteHeader(status)
	for i := range results {
		r := results[i]
		s := fmt.Sprintf("%s, %s|%s\n", r.City, r.Country, r.FQDN)
		rw.Write([]byte(s))
	}
}

// writeJSONLegacy supports the v1 result format, a single object for single results, or an array of objects for multiple results.
func writeJSONLegacy(rw http.ResponseWriter, status int, results v1.Results) {
	var b []byte
	var err error
	if len(results) == 1 {
		// Single results should be reported as a JSON object.
		b, err = json.Marshal(results[0])
	} else {
		// Multiple results should be reported as a JSON array.
		b, err = json.Marshal(results)
	}
	// Errors are only possible when marshalling incompatible types, like functions.
	rtx.PanicOnError(err, "Failed to format result")
	rw.WriteHeader(status)
	rw.Write(b)
}

// translate converts from the native Locate v2 result form to the v1.Results structure.
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

// getLegacyExperimentAndService converts a request path (e.g. /ndt) to a
// service known to Locate v2 (e.g. ndt/ndt5)
func getLegacyExperimentAndService(p string) (string, string) {
	service, ok := static.LegacyConvert[p]
	if !ok {
		return "", ""
	}
	datatype := path.Base(service)
	experiment := path.Base(path.Dir(service))
	return experiment, experiment + "/" + datatype
}
