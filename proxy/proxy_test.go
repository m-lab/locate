// Package proxy issues requests to the legacy mlab-ns service and parses responses.
package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/static"
)

var legacyNames = `[
  {
    "ip": [
      "128.177.119.203",
      "2001:438:fffd:2b::203"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-iupui-mlab1-lga06.measurement-lab.org",
    "site": "lga06"
  },
  {
    "ip": [
      "4.35.94.37",
      "2001:1900:2100:14::37"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-iupui-mlab3-lga05.measurement-lab.org",
    "site": "lga05"
  }
]`

var projectNames = `[
  {
    "ip": [
      "128.177.119.203",
      "2001:438:fffd:2b::203"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-mlab1-lga06.mlab-staging.measurement-lab.org",
    "site": "lga06"
  },
  {
    "ip": [
      "4.35.94.37",
      "2001:1900:2100:14::37"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-mlab3-lga05.mlab-staging.measurement-lab.org",
    "site": "lga05"
  }
]`

var shortNames = `[
  {
    "ip": [
      "64.86.148.152",
      "2001:5a0:4300::152"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-mlab2-lga03.measurement-lab.org",
    "site": "lga03"
  },
  {
    "ip": [
      "38.106.70.165",
      "2001:550:1d00:100::165"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "ndt-mlab3-lga08.measurement-lab.org",
    "site": "lga08"
  }
]`

var badNames = `[
  {
    "ip": [
      "64.86.148.152",
      "2001:5a0:4300::152"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "invalid-hostname.measurementlab.net",
    "site": "lga03"
  },
  {
    "ip": [
      "38.106.70.165",
      "2001:550:1d00:100::165"
    ],
    "country": "US",
    "city": "New York_NY",
    "fqdn": "invalid-hostname-2.measurementlab.net",
    "site": "lga08"
  }
]`

type fakeNS struct {
	content     []byte
	status      int
	breakReader bool
}

func (f *fakeNS) defaultHandler(rw http.ResponseWriter, req *http.Request) {
	if f.breakReader {
		// Speicyfing a content length larger than the actual response generates
		// a read error in the client.
		rw.Header().Set("Content-Length", "8000")
	}
	rw.WriteHeader(f.status)
	rw.Write([]byte(f.content))
}

func TestNearest(t *testing.T) {
	tests := []struct {
		name        string
		service     string
		lat         string
		lon         string
		content     string
		status      int
		breakReader bool
		badScheme   string
		badURL      string
		want        []string
		wantErr     bool
	}{
		{
			name:    "success-legacy-names",
			service: "ndt/ndt5",
			lat:     "40.3",
			lon:     "-70.1",
			content: legacyNames,
			status:  http.StatusOK,
			want: []string{
				"mlab1-lga06.measurement-lab.org", "mlab3-lga05.measurement-lab.org",
			},
		},
		{
			name:    "success-project-names",
			service: "ndt/ndt5",
			lat:     "40.3",
			lon:     "-70.1",
			content: projectNames,
			status:  http.StatusOK,
			want: []string{
				"mlab1-lga06.mlab-staging.measurement-lab.org", "mlab3-lga05.mlab-staging.measurement-lab.org",
			},
		},
		{
			name:    "success-short-names",
			service: "ndt/ndt5",
			lat:     "40.3",
			lon:     "-70.1",
			content: shortNames,
			status:  http.StatusOK,
			want: []string{
				"mlab2-lga03.measurement-lab.org", "mlab3-lga08.measurement-lab.org",
			},
		},
		{
			name:    "success-bad-names",
			service: "ndt/ndt5",
			lat:     "40.3",
			lon:     "-70.1",
			content: badNames,
			status:  http.StatusOK,
			want:    []string{},
		},
		{
			name:    "error-no-content",
			service: "ndt/ndt5",
			lat:     "40.3",
			lon:     "-70.1",
			content: "",
			status:  http.StatusNoContent,
			wantErr: true,
		},
		{
			name:        "bad-reader",
			service:     "ndt/ndt5",
			status:      http.StatusOK,
			breakReader: true,
			wantErr:     true,
		},
		{
			name:    "bad-service",
			service: "unknown-service-name",
			status:  http.StatusOK,
			wantErr: true,
		},
		{
			name:      "bad-request",
			service:   "ndt/ndt5",
			badScheme: ":",
			wantErr:   true,
		},
		{
			name:    "bad-url",
			service: "ndt/ndt5",
			badURL:  "http://fake/this-does-not-exist",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeNS{
				content:     []byte(tt.content),
				status:      tt.status,
				breakReader: tt.breakReader,
			}
			mux := http.NewServeMux()
			mux.HandleFunc("/"+static.LegacyServices[tt.service], f.defaultHandler)
			srv := httptest.NewServer(mux)
			p, err := url.Parse(srv.URL)
			rtx.Must(err, "failed to parse test server url")
			LegacyServer = *p
			if tt.badScheme != "" {
				// While a url with a bad scheme can be converted using .String(),
				// it will fail to parse again. This injects an error in NewRequestWithContext().
				LegacyServer.Scheme = ":"
			}
			if tt.badURL != "" {
				// Connections should fail to a bad url.
				p, err := url.Parse(tt.badURL)
				rtx.Must(err, "failed to parse test server url")
				LegacyServer = *p
			}

			ctx := context.Background()
			got, err := Nearest(ctx, tt.service, tt.lat, tt.lon)
			if (err != nil) != tt.wantErr {
				t.Errorf("Nearest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Nearest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_UnmarshalResponse(t *testing.T) {
	type fakeObject struct {
		Message string
	}
	tests := []struct {
		name        string
		url         string
		result      interface{}
		content     string
		status      int
		breakReader bool
		wantErr     bool
	}{
		{
			name:    "success",
			result:  &fakeObject{},
			content: `{"Message":"success"}`,
			status:  http.StatusOK,
		},
		{
			name:    "error-response",
			url:     "http://fake/this-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-no-content",
			content: "",
			status:  http.StatusNoContent,
			wantErr: true,
		},
		{
			name:        "error-reader",
			status:      http.StatusOK,
			breakReader: true,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeNS{
				content:     []byte(tt.content),
				status:      tt.status,
				breakReader: tt.breakReader,
			}
			srv := httptest.NewServer(http.HandlerFunc(f.defaultHandler))
			url := srv.URL
			if tt.url != "" {
				// Override url with test url.
				url = tt.url
			}

			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Errorf("failed to create request")
			}
			if err := UnmarshalResponse(req, tt.result); (err != nil) != tt.wantErr {
				t.Errorf("getRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			obj := tt.result.(*fakeObject)
			if obj.Message != "success" {
				t.Errorf("Result did not decode message: got %q, want 'success'", obj.Message)
			}
		})
	}
}
