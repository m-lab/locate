package testdata

import (
	"encoding/json"

	"github.com/m-lab/locate/cmd/heartbeat/messaging"
)

var (
	FakeHostname     = "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org"
	FakeRegistration = messaging.Registration{
		City:          "New York",
		CountryCode:   "US",
		ContinentCode: "NA",
		Experiment:    "",
		Hostname:      FakeHostname,
		Latitude:      40.7667,
		Longitude:     -73.8667,
		Machine:       "mlab1",
		Metro:         "lga",
		Project:       "mlab-sandbox",
		Site:          "lga0t",
		Type:          "physical",
		Uplink:        "10g",
	}
	EncodedRegistration = encodeMsg(FakeRegistration)

	FakeHealth = messaging.Health{
		Hostname: FakeHostname,
		Score:    1.0,
	}
	EncodedHealth = encodeMsg(FakeHealth)
)

func encodeMsg(msg interface{}) []byte {
	b, _ := json.Marshal(msg)
	return b
}
