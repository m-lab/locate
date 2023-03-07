// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	log "github.com/sirupsen/logrus"
	"gopkg.in/square/go-jose.v2/jwt"
)

func init() {
	// Disable most logs for unit tests.
	log.SetLevel(log.FatalLevel)
}

type fakeSigner struct {
	err error
}

func (s *fakeSigner) Sign(cl jwt.Claims) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	t := strings.Join([]string{
		cl.Audience[0], cl.Subject, cl.Issuer, cl.Expiry.Time().Format(time.RFC3339),
	}, "--")
	return t, nil
}

type fakeLocator struct {
	err     error
	targets []v2.Target
}

func (l *fakeLocator) Nearest(ctx context.Context, service, lat, lon string) ([]v2.Target, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.targets, nil
}

type fakeLocatorV2 struct {
	heartbeat.StatusTracker
	err     error
	targets []v2.Target
	urls    []url.URL
}

func (l *fakeLocatorV2) Nearest(service string, lat, lon float64, opts *heartbeat.NearestOptions) (*heartbeat.TargetInfo, error) {
	if l.err != nil {
		return nil, l.err
	}
	return &heartbeat.TargetInfo{
		Targets: l.targets,
		URLs:    l.urls,
		Ranks:   map[string]int{},
	}, nil
}

type fakeAppEngineLocator struct {
	loc *clientgeo.Location
	err error
}

func (l *fakeAppEngineLocator) Locate(req *http.Request) (*clientgeo.Location, error) {
	return l.loc, l.err
}

func TestClient_TranslatedQuery(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		signer     Signer
		locator    *fakeLocator
		project    string
		latlon     string
		header     http.Header
		wantLatLon string
		wantKey    string
		wantStatus int
	}{
		{
			name:       "error-bad-service",
			path:       "no-experiment-has-this/datatype-name",
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "error-nearest-failure",
			path: "ndt/ndt5",
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			locator: &fakeLocator{
				err: errors.New("Fake signer error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "error-nearest-failure-no-content",
			path: "ndt/ndt5",
			locator: &fakeLocator{
				err: proxy.ErrNoContent,
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:   "error-corrupt-latlon",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocator{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"corrupt-value"},
			},
			wantLatLon: "",
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:   "success-nearest-server",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocator{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-region",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocator{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-AppEngine-Country": []string{"US"},
				"X-AppEngine-Region":  []string{"ny"},
			},
			wantLatLon: "43.19880000,-75.3242000", // Region center.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-country",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocator{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-AppEngine-Region":      []string{"fake-region"},
				"X-AppEngine-Country":     []string{"US"},
				"X-AppEngine-CityLatLong": []string{"0.000000,0.000000"},
			},
			wantLatLon: "37.09024,-95.712891", // Country center.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := clientgeo.NewAppEngineLocator()
			c := NewClient(tt.project, tt.signer, tt.locator, &fakeLocatorV2{}, cl, prom.NewAPI(nil))

			mux := http.NewServeMux()
			mux.HandleFunc("/v2/nearest/", c.TranslatedQuery)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/nearest/"+tt.path+"?client_name=foo", nil)
			rtx.Must(err, "Failed to create request")
			req.Header = tt.header

			result := &v2.NearestResult{}
			resp, err := proxy.UnmarshalResponse(req, result)
			if err != nil {
				t.Fatalf("Failed to get response from: %s %s", srv.URL, tt.path)
			}
			if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("TranslatedQuery() wrong Access-Control-Allow-Origin header; got %s, want '*'",
					resp.Header.Get("Access-Control-Allow-Origin"))
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("TranslatedQuery() wrong Content-Type header; got %s, want 'application/json'",
					resp.Header.Get("Content-Type"))
			}
			if resp.Header.Get("X-Locate-ClientLatLon") != tt.wantLatLon {
				t.Errorf("TranslatedQuery() wrong X-Locate-ClientLatLon header; got %s, want '%s'",
					resp.Header.Get("X-Locate-ClientLatLon"), tt.wantLatLon)
			}
			if result.Error != nil && result.Error.Status != tt.wantStatus {
				t.Errorf("TranslatedQuery() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			if result.Error != nil {
				return
			}
			if result.Results == nil && tt.wantStatus == http.StatusOK {
				t.Errorf("TranslatedQuery() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			if len(tt.locator.targets) != len(result.Results) {
				t.Errorf("TranslateQuery() wrong result count; got %d, want %d",
					len(result.Results), len(tt.locator.targets))
			}
			if len(result.Results[0].URLs) != len(static.Configs[tt.path]) {
				t.Errorf("TranslateQuery() result wrong URL count; got %d, want %d",
					len(result.Results[0].URLs), len(static.Configs[tt.path]))
			}
			if _, ok := result.Results[0].URLs[tt.wantKey]; !ok {
				t.Errorf("TranslateQuery() result missing URLs key; want %q", tt.wantKey)
			}
		})
	}
}

func TestClient_Nearest(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		signer     Signer
		locator    *fakeLocatorV2
		cl         ClientLocator
		project    string
		latlon     string
		header     http.Header
		wantLatLon string
		wantKey    string
		wantStatus int
	}{
		{
			name:   "error-unmatched-service",
			path:   "no-instances-serve-this/datatype-name",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				err: errors.New("No servers found for this service error"),
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "error-nearest-failure",
			path: "ndt/ndt5",
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			locator: &fakeLocatorV2{
				err: errors.New("Fake signer error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "error-nearest-failure-no-content",
			path: "ndt/ndt5",
			locator: &fakeLocatorV2{
				err: heartbeat.ErrNoAvailableServers,
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "error-corrupt-latlon",
			path: "ndt/ndt5",
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"corrupt-value"},
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "error-cannot-parse-latlon",
			path: "ndt/ndt5",
			cl: &fakeAppEngineLocator{
				loc: &clientgeo.Location{
					Latitude:  "invalid-float",
					Longitude: "invalid-float",
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:   "success-nearest-server",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-region",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-Country": []string{"US"},
				"X-AppEngine-Region":  []string{"ny"},
			},
			wantLatLon: "43.19880000,-75.3242000", // Region center.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-country",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-Region":      []string{"fake-region"},
				"X-AppEngine-Country":     []string{"US"},
				"X-AppEngine-CityLatLong": []string{"0.000000,0.000000"},
			},
			wantLatLon: "37.09024,-95.712891", // Country center.
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cl == nil {
				tt.cl = clientgeo.NewAppEngineLocator()
			}
			c := NewClient(tt.project, tt.signer, &fakeLocator{}, tt.locator, tt.cl, prom.NewAPI(nil))

			mux := http.NewServeMux()
			mux.HandleFunc("/v2beta2/nearest/", c.Nearest)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/v2beta2/nearest/"+tt.path+"?client_name=foo", nil)
			rtx.Must(err, "Failed to create request")
			req.Header = tt.header

			result := &v2.NearestResult{}
			resp, err := proxy.UnmarshalResponse(req, result)
			if err != nil {
				t.Fatalf("Failed to get response from: %s %s", srv.URL, tt.path)
			}
			if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("Nearest() wrong Access-Control-Allow-Origin header; got %s, want '*'",
					resp.Header.Get("Access-Control-Allow-Origin"))
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Nearest() wrong Content-Type header; got %s, want 'application/json'",
					resp.Header.Get("Content-Type"))
			}
			if resp.Header.Get("X-Locate-ClientLatLon") != tt.wantLatLon {
				t.Errorf("Nearest() wrong X-Locate-ClientLatLon header; got %s, want '%s'",
					resp.Header.Get("X-Locate-ClientLatLon"), tt.wantLatLon)
			}
			if result.Error != nil && result.Error.Status != tt.wantStatus {
				t.Errorf("Nearest() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			if result.Error != nil {
				return
			}
			if result.Results == nil && tt.wantStatus == http.StatusOK {
				t.Errorf("Nearest() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			if len(tt.locator.targets) != len(result.Results) {
				t.Errorf("Nearest() wrong result count; got %d, want %d",
					len(result.Results), len(tt.locator.targets))
			}
			if len(result.Results[0].URLs) != len(static.Configs[tt.path]) {
				t.Errorf("Nearest() result wrong URL count; got %d, want %d",
					len(result.Results[0].URLs), len(static.Configs[tt.path]))
			}
			if _, ok := result.Results[0].URLs[tt.wantKey]; !ok {
				t.Errorf("Nearest() result missing URLs key; want %q", tt.wantKey)
			}
		})
	}
}

func TestNewClientDirect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c := NewClientDirect("fake-project", nil, nil, nil, nil, nil)
		if c == nil {
			t.Error("got nil client!")
		}
	})
}

func TestClient_Ready(t *testing.T) {
	tests := []struct {
		name       string
		fakeErr    error
		wantStatus int
	}{
		{
			name:       "success",
			wantStatus: http.StatusOK,
		},
		{
			name:       "error-not-ready",
			fakeErr:    errors.New("fake error"),
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient("foo", &fakeSigner{}, &fakeLocator{},
				&fakeLocatorV2{StatusTracker: &heartbeattest.FakeStatusTracker{Err: tt.fakeErr}}, nil, nil)

			mux := http.NewServeMux()
			mux.HandleFunc("/ready/", c.Ready)
			mux.HandleFunc("/live/", c.Live)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/ready", nil)
			rtx.Must(err, "Failed to create request")
			resp, err := http.DefaultClient.Do(req)
			rtx.Must(err, "failed to issue request")
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Ready() wrong status; got %d; want %d", resp.StatusCode, tt.wantStatus)
			}
			defer resp.Body.Close()

			req, err = http.NewRequest(http.MethodGet, srv.URL+"/live", nil)
			rtx.Must(err, "Failed to create request")
			resp, err = http.DefaultClient.Do(req)
			rtx.Must(err, "failed to issue request")
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Live() wrong status; got %d; want %d", resp.StatusCode, http.StatusOK)
			}
			defer resp.Body.Close()
		})
	}
}

func TestExtraParams(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		p        paramOpts
		want     url.Values
	}{
		{
			name:     "all-params",
			hostname: "host",
			p: paramOpts{
				raw:     map[string][]string{"client_name": {"client"}},
				version: "v2",
				ranks:   map[string]int{"host": 0},
			},
			want: url.Values{
				"client_name":    []string{"client"},
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
			},
		},
		{
			name:     "no-client",
			hostname: "host",
			p: paramOpts{
				version: "v2",
				ranks:   map[string]int{"host": 0},
			},
			want: url.Values{
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
			},
		},
		{
			name:     "unmatched-host",
			hostname: "host",
			p: paramOpts{
				version: "v2",
				ranks:   map[string]int{"different-host": 0},
			},
			want: url.Values{
				"locate_version": []string{"v2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extraParams(tt.hostname, tt.p)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extraParams() = %v, want %v", got, tt.want)
			}
		})
	}
}
