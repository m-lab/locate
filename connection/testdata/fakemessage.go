package testdata

import (
	v2 "github.com/m-lab/locate/api/v2"
)

var (
	FakeHostname     = "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org"
	FakeRegistration = v2.Registration{
		City:          "New York",
		CountryCode:   "US",
		ContinentCode: "NA",
		Experiment:    "ndt",
		Hostname:      FakeHostname,
		Latitude:      40.7667,
		Longitude:     -73.8667,
		Machine:       "mlab1",
		Metro:         "lga",
		Project:       "mlab-sandbox",
		Site:          "lga0t",
		Type:          "physical",
		Uplink:        "10g",
		Services: map[string][]string{
			"ndt/ndt7": []string{
				"ws://ndt/v7/upload",
				"ws://ndt/v7/download",
				"wss://ndt/v7/upload",
				"wss://ndt/v7/download",
			},
		},
	}
	FakeHealth = v2.Health{
		Hostname: FakeHostname,
		Score:    1.0,
	}
)
