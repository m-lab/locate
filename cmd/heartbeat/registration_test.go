package main

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/host"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var (
	validHostname           = "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org"
	validHostnameWithSuffix = "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org-t95j"
	validURL                = "file:./testdata/registration.json"
	validMsg                = &v2.Registration{
		City:          "New York",
		CountryCode:   "US",
		ContinentCode: "NA",
		Experiment:    "",
		Hostname:      "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org",
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

func Test_NewLoader(t *testing.T) {
	ticker, err := memoryless.NewTicker(context.Background(), memoryless.Config{
		Min:      static.RegistrationLoadMin,
		Expected: static.RegistrationLoadExpected,
		Max:      static.RegistrationLoadMax,
	})
	testingx.Must(t, err, "could not create ticker")

	tests := []struct {
		name     string
		url      *url.URL
		hostname string
		config   memoryless.Config
		want     *loader
		wantErr  bool
	}{
		{
			name: "success",
			url: &url.URL{
				Scheme: "file",
				Opaque: "./testdata/registration.json",
			},
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org",
			config: memoryless.Config{
				Min:      static.RegistrationLoadMin,
				Expected: static.RegistrationLoadExpected,
				Max:      static.RegistrationLoadMax,
			},
			want: &loader{
				url: &url.URL{
					Scheme: "file",
					Opaque: "./testdata/registration.json",
				},
				ticker: ticker,
				hostname: host.Name{
					Service: "ndt",
					Machine: "mlab1",
					Site:    "lga0t",
					Project: "mlab-sandbox",
					Domain:  "measurement-lab.org",
					Suffix:  "",
					Version: "v2",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid-hostname",
			url: &url.URL{
				Scheme: "file",
				Opaque: "./testdata/registration.json",
			},
			hostname: "foo",
			config: memoryless.Config{
				Min:      static.RegistrationLoadMin,
				Expected: static.RegistrationLoadExpected,
				Max:      static.RegistrationLoadMax,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:     "nil-url",
			url:      nil,
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org",
			config: memoryless.Config{
				Min:      static.RegistrationLoadMin,
				Expected: static.RegistrationLoadExpected,
				Max:      static.RegistrationLoadMax,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid-config",
			url: &url.URL{
				Scheme: "file",
				Opaque: "./testdata/registration.json",
			},
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org",
			config: memoryless.Config{
				Min: 1,
				Max: -1,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := NewLoader(tt.url, tt.hostname, "", map[string][]string{}, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLoader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := deep.Equal(got, tt.want); diff != nil {
				t.Errorf("NewLoader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetRegistration(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		url          string
		savedReg     v2.Registration
		wantErr      bool
		wantMsg      *v2.Registration
		wantSavedReg v2.Registration
	}{
		{
			name:         "valid-data",
			url:          validURL,
			hostname:     validHostname,
			wantErr:      false,
			wantMsg:      validMsg,
			wantSavedReg: *validMsg,
		},
		{
			name:         "valid-hostname-with-suffix",
			url:          validURL,
			hostname:     validHostname + "-t95j",
			wantErr:      false,
			wantMsg:      validMsg,
			wantSavedReg: *validMsg,
		},
		{
			name:         "empty-url",
			hostname:     validHostname,
			wantErr:      true,
			wantMsg:      nil,
			wantSavedReg: v2.Registration{},
		},
		{
			name:         "invalid-url-scheme",
			hostname:     validHostname,
			url:          "foo:./testdata/registration.json",
			wantErr:      true,
			wantMsg:      nil,
			wantSavedReg: v2.Registration{},
		},
		{
			name:         "invalid-registration",
			hostname:     validHostname,
			url:          "file:./testdata/invalid",
			wantErr:      true,
			wantMsg:      nil,
			wantSavedReg: v2.Registration{},
		},
		{
			name:         "non-existent-file",
			hostname:     validHostname,
			url:          "file:./testdata/non-existent.json",
			wantErr:      true,
			wantMsg:      nil,
			wantSavedReg: v2.Registration{},
		},
		{
			name:         "hostname-not-found",
			hostname:     "ndt-mlab1-pdx0t.mlab-sandbox.measurement-lab.org",
			url:          validURL,
			wantErr:      true,
			wantMsg:      nil,
			wantSavedReg: v2.Registration{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.url)
			testingx.Must(t, err, "could not parse URL")

			h, err := host.Parse(tt.hostname)
			testingx.Must(t, err, "could not parse hostname")

			ldr := &loader{
				url:      u,
				hostname: h,
				reg:      tt.savedReg,
			}
			gotMsg, gotErr := ldr.GetRegistration(context.Background())

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("GetRegistration() error: %v, want: %v", gotErr, tt.wantErr)
			}

			if diff := deep.Equal(gotMsg, tt.wantMsg); diff != nil {
				t.Errorf("GetRegistration() message did not match; got: %+v, want: %+v", gotMsg, tt.wantMsg)
			}

			if diff := deep.Equal(ldr.reg, tt.wantSavedReg); diff != nil {
				t.Errorf("GetRegistration() saved registration did not match; got: %+v, want: %+v", ldr.reg, tt.wantSavedReg)
			}
		})
	}
}
