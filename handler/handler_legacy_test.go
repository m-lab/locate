package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/apex/log"
	"github.com/m-lab/go/rtx"
	v1 "github.com/m-lab/locate/api/v1"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/limits"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

func init() {
	// disable apex logging for tests.
	log.SetLevel(log.FatalLevel)
}

func TestClient_LegacyNearest(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		serverPath string
		signer     Signer
		locator    *fakeLocatorV2
		cl         ClientLocator
		project    string
		latlon     string
		limits     limits.Agents
		header     http.Header
		wantLatLon string
		wantStatus int
	}{
		{
			name: "error-nearest-failure",
			path: "ndt",
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
			name:       "error-nearest-bad-experiment",
			path:       "ndtx",
			serverPath: "/ndtx",
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			locator: &fakeLocatorV2{
				err: errors.New("Fake signer error"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "error-nearest-failure-no-content",
			path: "ndt",
			locator: &fakeLocatorV2{
				err: heartbeat.ErrNoAvailableServers,
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "error-corrupt-latlon",
			path: "ndt",
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"corrupt-value"},
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "error-cannot-parse-latlon",
			path: "ndt",
			cl: &fakeAppEngineLocator{
				loc: &clientgeo.Location{
					Latitude:  "invalid-float",
					Longitude: "invalid-float",
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "error-limit-request",
			path: "ndt",
			limits: limits.Agents{
				"foo": limits.NewCron("* * * * *", time.Minute),
			},
			header: http.Header{
				"User-Agent": []string{"foo"},
			},
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:   "success-nearest-server",
			path:   "ndt",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{}}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-bt",
			path:   "ndt?format=bt",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{City: "New York", Country: "US"}}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-no-servers",
			path:   "ndt",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "skip-bad-host-not-a-real-name", Location: &v2.Location{}}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "success-nearest-servers",
			path:   "ndt?policy=geo_options",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{}},
					{Machine: "mlab2-lga0t.measurement-lab.org", Location: &v2.Location{}}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
			},
			wantLatLon: "40.3,-70.4", // Client receives lat/lon provided by AppEngine.
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-random",
			path:   "ndt?policy=random",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{}}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "ndt_protocol"},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-region",
			path:   "ndt",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{}}},
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
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-nearest-server-using-country",
			path:   "ndt",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{
					{Machine: "mlab1-lga0t.measurement-lab.org", Location: &v2.Location{}}},
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
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cl == nil {
				tt.cl = clientgeo.NewAppEngineLocator()
			}
			if tt.serverPath == "" {
				tt.serverPath = "/ndt"
			}
			c := NewClient(tt.project, tt.signer, tt.locator, tt.cl, prom.NewAPI(nil), tt.limits)

			mux := http.NewServeMux()
			mux.HandleFunc(tt.serverPath, c.LegacyNearest)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/"+tt.path, nil)
			rtx.Must(err, "Failed to create request")
			req.Header = tt.header

			resp, results, err := parseV1Results(req)
			if err != nil {
				t.Fatalf("Failed to get response from: %#v - %s %s", err, srv.URL, tt.path)
			}
			if tt.wantStatus != http.StatusOK {
				if len(results) == 0 && resp.StatusCode != tt.wantStatus {
					t.Errorf("LegacyNearest() wrong status; got %d, want %d", resp.StatusCode, tt.wantStatus)
				}
				return
			}
			if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("LegacyNearest() wrong Access-Control-Allow-Origin header; got %s, want '*'",
					resp.Header.Get("Access-Control-Allow-Origin"))
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("LegacyNearest() wrong Content-Type header; got %s, want 'application/json'",
					resp.Header.Get("Content-Type"))
			}
			if resp.Header.Get("X-Locate-ClientLatLon") != tt.wantLatLon {
				t.Errorf("LegacyNearest() wrong X-Locate-ClientLatLon header; got %s, want '%s'",
					resp.Header.Get("X-Locate-ClientLatLon"), tt.wantLatLon)
			}
			if len(results) != 0 && resp.StatusCode != tt.wantStatus {
				t.Errorf("LegacyNearest() wrong status; got %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if len(tt.locator.targets) != len(results) {
				t.Errorf("LegacyNearest() wrong result count; got %d, want %d",
					len(results), len(tt.locator.targets))
			}
		})
	}
}

func parseV1Results(req *http.Request) (*http.Response, v1.Results, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, nil, nil
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	if len(b) == 0 {
		return resp, nil, nil
	}
	s := string(b)
	var r interface{}
	// Peek at the content to infer the result type.
	if s[0] == '[' {
		r = &v1.Results{}
	} else if s[0] == '{' {
		r = &v1.Result{}
	} else {
		// Since there is content, assume it's the "BT" format for tests.
		s = strings.TrimSpace(s)
		f := strings.Split(s, "|")
		cc := strings.Split(f[0], ", ")
		results := v1.Results{
			v1.Result{
				City:    cc[0],
				Country: cc[1],
				FQDN:    f[1],
			},
		}
		return resp, results, nil
	}
	err = json.Unmarshal(b, r)
	if err != nil {
		return resp, nil, err
	}
	// Return a type of v1.Results unconditionally.
	var results v1.Results
	switch v := r.(type) {
	case *v1.Result:
		results = v1.Results{*v}
	case *v1.Results:
		results = *v
	default:
		panic(v)
	}
	return resp, results, nil
}
