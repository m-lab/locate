package v2

import (
	"encoding/json"
	"testing"

	"github.com/go-test/deep"
	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/go/testingx"
)

var tests = []struct {
	name     string
	receiver redis.Scanner
	scanObj  interface{}
}{
	{
		name:     "registration-success",
		receiver: &Registration{},
		scanObj: &Registration{
			City:          "New York",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org",
			Latitude:      40.7667,
			Longitude:     -73.8667,
			Machine:       "mlab1",
			Metro:         "lga",
			Project:       "mlab-sandbox",
			Site:          "lga0t",
			Type:          "physical",
			Uplink:        "10g",
			Services: map[string][]string{
				"ndt/ndt7": {
					"ws://ndt/v7/upload",
					"ws://ndt/v7/download",
					"wss://ndt/v7/upload",
					"wss://ndt/v7/download",
				},
			},
		},
	},
	{
		name:     "health-success",
		receiver: &Health{},
		scanObj: &Health{
			Score: 1.0,
		},
	},
}

func TestRedisScan_Success(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.scanObj)
			testingx.Must(t, err, "failed to marshal obj")

			err = tt.receiver.RedisScan(b)
			if err != nil {
				t.Fatalf("RedisScan() error: %+v, want: nil", err)
			}

			if diff := deep.Equal(tt.receiver, tt.scanObj); diff != nil {
				t.Errorf("RedisScan() failed to scan object; got: %+v. want: %+v", tt.receiver,
					tt.scanObj)
			}
		})
	}
}

func TestRedisScan_Failure(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.receiver.RedisScan(tt.scanObj)
			if err == nil {
				t.Error("RedisScan() error: nil, want: err")
			}
		})
	}
}
