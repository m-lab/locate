package siteinfo

import (
	"fmt"
	"net/url"

	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// Hosts returns a map of hosts that Locate knows about. The map values
// are a combination of an experiment's heartbeat registration information and
// health informatiom from both heartbeat and Prometheus.
func Hosts(msgs map[string]v2.HeartbeatMessage, v url.Values) (map[string]v2.HeartbeatMessage, error) {
	hosts, err := filterHosts(msgs, v)
	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// Geo returns a geojson.FeatureCollection representing all machines that Locate
// knows about.
func Geo(msgs map[string]v2.HeartbeatMessage, v url.Values) (*geojson.FeatureCollection, error) {
	var fc *geojson.FeatureCollection
	var f *geojson.Feature

	fc = geojson.NewFeatureCollection()

	hosts, err := filterHosts(msgs, v)
	if err != nil {
		return nil, err
	}

	for k, h := range hosts {
		parts, err := host.Parse(k)
		if err != nil {
			returnError := fmt.Errorf("failed to parse hostname: %s", k)
			return fc, returnError
		}

		// We have witnessed a case in staging where a v2.HeartbeatMessage's
		// Registration field was nil for some reason. It's not clear under
		// what circumstances this error condition can happen, but skip this
		// host if Registration is nil to avoid panics when trying to
		// dereference a nil pointer.
		if h.Registration == nil {
			continue
		}

		f = geojson.NewFeature(orb.Point{h.Registration.Longitude, h.Registration.Latitude})
		f.Properties = map[string]interface{}{
			"health":      h.Health.Score,
			"hostname":    h.Registration.Hostname,
			"machine":     fmt.Sprintf("%s-%s", parts.Site, parts.Machine),
			"org":         parts.Org,
			"probability": h.Registration.Probability,
			"uplink":      h.Registration.Uplink,
			"type":        h.Registration.Type,
		}

		fc.Append(f)
	}

	return fc, nil
}

// filterHosts filters a list of v2.HeartbeatMessages based on the organization
// or experiment that was passed as query paramters with the request.
func filterHosts(msgs map[string]v2.HeartbeatMessage, v url.Values) (map[string]v2.HeartbeatMessage, error) {
	org := v.Get("org")
	exp := v.Get("exp")

	// If not filters are specified then just return msgs unmodified.
	if org == "" && exp == "" {
		return msgs, nil
	}

	hosts := make(map[string]v2.HeartbeatMessage)

	for k, msg := range msgs {
		parts, err := host.Parse(k)
		if err != nil {
			returnError := fmt.Errorf("failed to parse hostname: %s", k)
			return nil, returnError
		}

		// Skip if org filter is specified but doesn't match
		if org != "" && org != parts.Org {
			continue
		}

		// Skip if exp filter is specified but doesn't match
		if exp != "" && exp != parts.Service {
			continue
		}

		// If we get here, all specified filters match
		hosts[k] = msg
	}

	return hosts, nil
}
