package registration

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-test/deep"
)

var (
	validHostname = "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org"
	validURL      = "file:./testdata/registration.json"
	validMsg      = &RegistrationMessage{
		City:          "New York",
		CountryCode:   "US",
		ContinentCode: "NA",
		Experiment:    "",
		Hostname:      validHostname,
		Latitude:      40.7667,
		Longitude:     -73.8667,
		Machine:       "mlab1",
		Metro:         "lga",
		Project:       "mlab-sandbox",
		Site:          "lga0t",
		Type:          "physical",
		Uplink:        "10g",
	}
)

func Test_Load(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		url      string
		wantErr  bool
		wantMsg  *RegistrationMessage
	}{
		{
			name:     "valid-data",
			hostname: validHostname,
			url:      validURL,
			wantErr:  false,
			wantMsg:  validMsg,
		},
		{
			name:     "invalid-hostname",
			hostname: "foo",
			url:      validURL,
			wantErr:  true,
			wantMsg:  nil,
		},
		{
			name:     "empty-url",
			hostname: validHostname,
			wantErr:  true,
			wantMsg:  nil,
		},
		{
			name:     "invalid-url-scheme",
			hostname: validHostname,
			url:      "foo:./testdata/registration.json",
			wantErr:  true,
			wantMsg:  nil,
		},
		{
			name:     "invalid-registration",
			hostname: validHostname,
			url:      "file:./testdata/invalid",
			wantErr:  true,
			wantMsg:  nil,
		},
		{
			name:     "hostname-not-found",
			hostname: "ndt-mlab1-pdx0t.mlab-sandbox.measurement-lab.org",
			url:      validURL,
			wantErr:  true,
			wantMsg:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse(tt.url)
			gotMsg, gotErr := Load(context.Background(), tt.hostname, u)

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("Load() error: %v, want: %v", gotErr, tt.wantErr)
			}

			if diff := deep.Equal(gotMsg, tt.wantMsg); diff != nil {
				t.Errorf("Load() message did not match; got: %+v, want: %+v", gotMsg, tt.wantMsg)
			}
		})
	}
}
