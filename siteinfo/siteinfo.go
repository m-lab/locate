package siteinfo

import (
	"fmt"
	"net/url"

	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
)

// Machines returns a map of machines that Locate knows about. The map values
// are a combination of a machine's heartbeat registration information and
// health informatiom from both heartbeat and Prometheus.
func Machines(msgs map[string]v2.HeartbeatMessage, v url.Values) (map[string]v2.HeartbeatMessage, error) {
	machines := make(map[string]v2.HeartbeatMessage)

	org := v.Get("org")
	exp := v.Get("exp")

	if org != "" || exp != "" {
		for k, v := range msgs {
			parts, err := host.Parse(k)
			if err != nil {
				returnError := fmt.Errorf("failed to parse hostname: %s", k)
				return nil, returnError
			}
			if org != "" && exp == "" {
				if org == parts.Org {
					machines[k] = v
				}
			} else if org == "" && exp != "" {
				if exp == parts.Service {
					machines[k] = v
				}
			} else {
				if org == parts.Org && exp == parts.Service {
					machines[k] = v
				}
			}
		}
	} else {
		machines = msgs
	}

	return machines, nil

}

// Geo returns a geojson.FeatureCollection representing all machines that Locate
// knows about.
func Geo(msgs map[string]v2.HeartbeatMessage) (*geojson.FeatureCollection, error) {
	var fc *geojson.FeatureCollection
	var f *geojson.Feature

	fc = geojson.NewFeatureCollection()

	for k, v := range msgs {
		parts, err := host.Parse(k)
		if err != nil {
			returnError := fmt.Errorf("failed to parse hostname: %s", k)
			return fc, returnError
		}

		f = geojson.NewFeature(orb.Point{v.Registration.Latitude, v.Registration.Longitude})
		f.Properties = map[string]interface{}{
			"health":      v.Health.Score,
			"name":        v.Registration.Site,
			"org":         parts.Org,
			"probability": v.Registration.Probability,
			"uplink":      v.Registration.Uplink,
			"type":        v.Registration.Type,
		}

		fc.Append(f)
	}

	return fc, nil
}
