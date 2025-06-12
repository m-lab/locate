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

	for k, v := range hosts {
		parts, err := host.Parse(k)
		if err != nil {
			returnError := fmt.Errorf("failed to parse hostname: %s", k)
			return fc, returnError
		}

		f = geojson.NewFeature(orb.Point{v.Registration.Longitude, v.Registration.Latitude})
		f.Properties = map[string]interface{}{
			"health":      v.Health.Score,
			"hostname":    v.Registration.Hostname,
			"machine":     fmt.Sprintf("%s-%s", parts.Site, parts.Machine),
			"org":         parts.Org,
			"probability": v.Registration.Probability,
			"uplink":      v.Registration.Uplink,
			"type":        v.Registration.Type,
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
