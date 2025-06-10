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
	"msak-chs9999-ab285f12.otherorg.sandbox.measurement-lab.org": {
		Health: &v2.Health{
			Score: 1,
		},
		Registration: &v2.Registration{
			City:          "Charleston",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "msak",
			Hostname:      "msak-chs9999-ab285f12.otherorg.sandbox.measurement-lab.org",
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
	"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org": {
		Health: &v2.Health{
			Score: 1,
		},
		Registration: &v2.Registration{
			City:          "Memphis",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
			Latitude:      32.8986,
			Longitude:     -80.0405,
			Machine:       "mlab1",
			Metro:         "abc",
			Project:       "mlab-sandbox",
			Probability:   1,
			Site:          "abc0t",
			Type:          "physical",
			Uplink:        "10g",
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

func TestFilterHosts(t *testing.T) {
	tests := []struct {
		name         string
		params       url.Values
		expectedKeys []string
		instances    map[string]v2.HeartbeatMessage
		wantErr      bool
	}{
		{
			name:      "success-return-all-records",
			instances: testInstances,
			expectedKeys: []string{
				"msak-chs9999-ab285f12.otherorg.sandbox.measurement-lab.org",
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name:      "success-return-mlab-records",
			instances: testInstances,
			params: url.Values{
				"org": {
					"mlab",
				},
			},
			expectedKeys: []string{
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name:      "success-return-ndt-records",
			instances: testInstances,
			params: url.Values{
				"exp": {
					"ndt",
				},
			},
			expectedKeys: []string{
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name:      "success-return-mlab-ndt-records",
			instances: testInstances,
			params: url.Values{
				"exp": {
					"ndt",
				},
				"org": {
					"mlab",
				},
			},
			expectedKeys: []string{
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name: "error-invalid-hostname",
			instances: map[string]v2.HeartbeatMessage{
				"invalid.hostname": {},
			},
			params: url.Values{
				"org": {
					"mlab",
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		var resultKeys []string

		result, err := filterHosts(test.instances, test.params)
		if (err != nil) != test.wantErr {
			t.Errorf("filterHosts() error = %v, wantErr %v", err, test.wantErr)
		}

		for k := range result {
			resultKeys = append(resultKeys, k)
		}

		sort.Strings(resultKeys)

		if !reflect.DeepEqual(test.expectedKeys, resultKeys) {
			t.Errorf("filterHosts() wanted = %v, got %v", test.expectedKeys, resultKeys)
		}
	}
}

func TestHosts(t *testing.T) {
	tests := []struct {
		name          string
		expectedHosts []string
		instances     map[string]v2.HeartbeatMessage
		wantErr       bool
	}{
		{
			name:      "success",
			instances: testInstances,
			expectedHosts: []string{
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name: "error",
			instances: map[string]v2.HeartbeatMessage{
				"invalid.hostname": {},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		var resultKeys []string

		params := url.Values{
			"exp": []string{"ndt"},
		}

		result, err := Hosts(test.instances, params)
		if (err != nil) != test.wantErr {
			t.Errorf("Hosts() error = %v, wantErr %v", err, test.wantErr)
		}

		for k := range result {
			resultKeys = append(resultKeys, k)
		}

		sort.Strings(resultKeys)

		if !reflect.DeepEqual(test.expectedHosts, resultKeys) {
			t.Errorf("Hosts() wanted = %v, got %v", test.expectedHosts, resultKeys)
		}
	}
}

func TestGeo(t *testing.T) {
	tests := []struct {
		name          string
		expectedHosts []string
		instances     map[string]v2.HeartbeatMessage
		wantErr       bool
	}{
		{
			name:      "success",
			instances: testInstances,
			expectedHosts: []string{
				"msak-chs9999-ab285f12.otherorg.sandbox.measurement-lab.org",
				"ndt-dfw8888-73a354f1.testorg.sandbox.measurement-lab.org",
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org",
				"ndt-oma7777-217f832a.mlab.sandbox.measurement-lab.org",
			},
		},
		{
			name: "error",
			instances: map[string]v2.HeartbeatMessage{
				"invalid.hostname": {},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		var resultFeatures []string

		params := url.Values{}

		result, err := Geo(test.instances, params)
		if (err != nil) != test.wantErr {
			t.Errorf("Geo() error = %v, wantErr %v", err, test.wantErr)
		}

		for _, v := range result.Features {
			resultFeatures = append(resultFeatures, v.Properties.MustString("hostname"))
		}

		sort.Strings(resultFeatures)

		if !reflect.DeepEqual(test.expectedHosts, resultFeatures) {
			t.Errorf("Geo() wanted = %v, got %v", test.expectedHosts, resultFeatures)
		}
	}
}
