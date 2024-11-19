package siteinfo

import (
	"net/url"
	"reflect"
	"sort"
	"testing"

	v2 "github.com/m-lab/locate/api/v2"
)

var testInstances = map[string]v2.HeartbeatMessage{
	"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org": {
		Health: &v2.Health{
			Score: 1,
		},
		Registration: &v2.Registration{
			City:          "Omaha",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			Latitude:      41.3032,
			Longitude:     -95.8941,
			Machine:       "217f832a",
			Metro:         "oma",
			Project:       "mlab-sandbox",
			Probability:   0.1,
			Site:          "oma7777",
			Type:          "unknown",
			Uplink:        "unknown",
			Services: map[string][]string{
				"ndt/ndt7": {
					"ws:///ndt/v7/download",
					"ws:///ndt/v7/upload",
					"wss:///ndt/v7/download",
					"wss:///ndt/v7/upload",
				},
			},
		},
		Prometheus: &v2.Prometheus{
			Health: true,
		},
	},
	"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org": {
		Health: &v2.Health{
			Score: 1,
		},
		Registration: &v2.Registration{
			City:          "Dallas",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
			Latitude:      32.8969,
			Longitude:     -97.0381,
			Machine:       "73a354f1",
			Metro:         "dfw",
			Project:       "mlab-sandbox",
			Probability:   0.1,
			Site:          "dfw8888",
			Type:          "unknown",
			Uplink:        "unknown",
			Services: map[string][]string{
				"ndt/ndt7": {
					"ws:///ndt/v7/download",
					"ws:///ndt/v7/upload",
					"wss:///ndt/v7/download",
					"wss:///ndt/v7/upload",
				},
			},
		},
		Prometheus: &v2.Prometheus{
			Health: true,
		},
	},
	"msak-chs9999-ab285f12.mlab.sandbox.measurement-lab.org": {
		Health: &v2.Health{
			Score: 1,
		},
		Registration: &v2.Registration{
			City:          "Charleston",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "msak",
			Hostname:      "msak-chs9999-ab285f12.mlab.sandbox.measurement-lab.org",
			Latitude:      32.8986,
			Longitude:     -80.0405,
			Machine:       "ab285f12",
			Metro:         "chs",
			Project:       "mlab-sandbox",
			Probability:   0.5,
			Site:          "chs9999",
			Type:          "unknown",
			Uplink:        "unknown",
			Services: map[string][]string{
				"ndt/ndt7": {
					"ws:///ndt/v7/download",
					"ws:///ndt/v7/upload",
					"wss:///ndt/v7/download",
					"wss:///ndt/v7/upload",
				},
			},
		},
		Prometheus: &v2.Prometheus{
			Health: false,
		},
	},
}

func TestMachines(t *testing.T) {
	tests := []struct {
		name         string
		params       url.Values
		expectedKeys []string
		instances    map[string]v2.HeartbeatMessage
		wantErr      bool
	}{
		{
			name: "success-return-all-records",
			expectedKeys: []string{
				"msak-chs9999-ab285f12.mlab.sandbox.measurement-lab.org",
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
			instances: testInstances,
			wantErr:   false,
		},
		{
			name: "success-return-mlab-records",
			params: url.Values{
				"org": {
					"mlab",
				},
			},
			expectedKeys: []string{
				"msak-chs9999-ab285f12.mlab.sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
			instances: testInstances,
			wantErr:   false,
		},
		{
			name: "success-return-ndt-records",
			params: url.Values{
				"exp": {
					"ndt",
				},
			},
			expectedKeys: []string{
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
			instances: testInstances,
			wantErr:   false,
		},
		{
			name: "success-return-mlab-ndt-records",
			params: url.Values{
				"exp": {
					"ndt",
				},
				"org": {
					"mlab",
				},
			},
			expectedKeys: []string{
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
			instances: testInstances,
			wantErr:   false,
		},
		{
			name: "error-invalid-hostname",
			params: url.Values{
				"org": {
					"mlab",
				},
			},
			instances: map[string]v2.HeartbeatMessage{
				"invalid.hostname": {},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		var resultKeys []string

		result, err := Machines(test.instances, test.params)
		if (err != nil) != test.wantErr {
			t.Errorf("Machines() error = %v, wantErr %v", err, test.wantErr)
		}

		for k := range result {
			resultKeys = append(resultKeys, k)
		}

		sort.Strings(resultKeys)

		if !reflect.DeepEqual(test.expectedKeys, resultKeys) {
			t.Errorf("Machines() wanted = %v, got %v", test.expectedKeys, resultKeys)
		}
	}
}
