// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	"github.com/m-lab/locate/limits"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	log "github.com/sirupsen/logrus"
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

type fakeRateLimiter struct {
	status limits.LimitStatus
	err    error
}

func (r *fakeRateLimiter) IsLimited(ip, ua string) (limits.LimitStatus, error) {
	if r.err != nil {
		return limits.LimitStatus{}, r.err
	}
	return r.status, nil
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
		limits     limits.Agents
		ipLimiter  Limiter
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
			name: "error-limit-request",
			path: "ndt/ndt5",
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
		{
			name:   "error-rate-limit-exceeded-ip",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-Forwarded-For": []string{"192.0.2.1"},
				"User-Agent":      []string{"test-client"},
			},
			ipLimiter: &fakeRateLimiter{
				status: limits.LimitStatus{
					IsLimited: true,
					LimitType: "ip",
				},
			},
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:   "error-rate-limit-exceeded-ipua",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			header: http.Header{
				"X-Forwarded-For": []string{"192.0.2.1"},
				"User-Agent":      []string{"test-client"},
			},
			ipLimiter: &fakeRateLimiter{
				status: limits.LimitStatus{
					IsLimited: true,
					LimitType: "ipua",
				},
			},
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:   "success-rate-limit-not-exceeded",
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
				"X-Forwarded-For":         []string{"192.168.1.1"},
				"User-Agent":              []string{"test-client"},
			},
			ipLimiter: &fakeRateLimiter{
				status: limits.LimitStatus{
					IsLimited: false,
					LimitType: "",
				},
			},
			wantLatLon: "40.3,-70.4",
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-rate-limiter-error",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "/ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
				"X-Forwarded-For":         []string{"192.168.1.1"},
				"User-Agent":              []string{"test-client"},
			},
			ipLimiter: &fakeRateLimiter{
				err: errors.New("redis error"),
			},
			wantLatLon: "40.3,-70.4",
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK, // Should fail open
		},
		{
			name:   "success-missing-forwarded-for",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
					{Scheme: "wss", Host: ":3010", Path: "/ndt_protocol"},
				},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
				// No X-Forwarded-For
				"User-Agent": []string{"test-client"},
			},
			ipLimiter: &fakeRateLimiter{
				status: limits.LimitStatus{
					IsLimited: false,
					LimitType: "",
				},
			},
			wantLatLon: "40.3,-70.4",
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cl == nil {
				tt.cl = clientgeo.NewAppEngineLocator()
			}
			c := NewClient(tt.project, tt.signer, tt.locator, tt.cl, prom.NewAPI(nil), tt.limits, nil, tt.ipLimiter, nil, nil)

			mux := http.NewServeMux()
			mux.HandleFunc("/v2/nearest/", c.Nearest)
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
		c := NewClientDirect("fake-project", nil, nil, nil, nil)
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
			c := NewClient("foo", &fakeSigner{}, &fakeLocatorV2{StatusTracker: &heartbeattest.FakeStatusTracker{Err: tt.fakeErr}}, nil, nil, nil, nil, nil, nil, nil)

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
func TestClient_Registrations(t *testing.T) {
	tests := []struct {
		name       string
		instances  map[string]v2.HeartbeatMessage
		fakeErr    error
		wantStatus int
	}{
		{
			name: "success-status-200",
			instances: map[string]v2.HeartbeatMessage{
				"ndt-mlab1-abc0t.mlab-sandbox.measurement-lab.org": {},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "error-status-500",
			instances: map[string]v2.HeartbeatMessage{
				"invalid-hostname.xyz": {},
			},
			fakeErr:    errors.New("fake error"),
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		fakeStatusTracker := &heartbeattest.FakeStatusTracker{
			Err:           tt.fakeErr,
			FakeInstances: tt.instances,
		}

		t.Run(tt.name, func(t *testing.T) {
			c := NewClient("foo", &fakeSigner{}, &fakeLocatorV2{StatusTracker: fakeStatusTracker}, nil, nil, nil, nil, nil, nil, nil)

			mux := http.NewServeMux()
			mux.HandleFunc("/v2/siteinfo/registrations/", c.Registrations)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/siteinfo/registrations?org=mlab", nil)
			rtx.Must(err, "failed to create request")
			resp, err := http.DefaultClient.Do(req)
			rtx.Must(err, "failed to issue request")
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Registrations() wrong status; got %d; want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

func TestExtraParams(t *testing.T) {
	tests := []struct {
		name                 string
		hostname             string
		index                int
		p                    paramOpts
		client               *Client
		earlyExitProbability float64
		want                 url.Values
	}{
		{
			name:     "all-params",
			hostname: "host",
			index:    0,
			p: paramOpts{
				raw:       map[string][]string{"client_name": {"client"}},
				version:   "v2",
				ranks:     map[string]int{"host": 0},
				svcParams: map[string]float64{},
			},
			client: &Client{},
			want: url.Values{
				"client_name":    []string{"client"},
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
				"index":          []string{"0"},
			},
		},
		{
			name:     "early-exit-client-match",
			hostname: "host",
			index:    0,
			p: paramOpts{
				raw:       map[string][]string{"client_name": {"foo"}},
				version:   "v2",
				ranks:     map[string]int{"host": 0},
				svcParams: map[string]float64{},
			},
			client: &Client{
				earlyExitClients: map[string]bool{"foo": true},
			},
			want: url.Values{
				"client_name":    []string{"foo"},
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
				"index":          []string{"0"},
				"early_exit":     []string{"250"},
			},
		},
		{
			name:     "early-exit-client-no-match",
			hostname: "host",
			index:    0,
			p: paramOpts{
				raw:       map[string][]string{"client_name": {"bar"}},
				version:   "v2",
				ranks:     map[string]int{"host": 0},
				svcParams: map[string]float64{},
			},
			client: &Client{
				earlyExitClients: map[string]bool{"foo": true},
			},
			want: url.Values{
				"client_name":    []string{"bar"},
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
				"index":          []string{"0"},
			},
		},
		{
			name:     "no-client",
			hostname: "host",
			index:    0,
			p: paramOpts{
				version:   "v2",
				ranks:     map[string]int{"host": 0},
				svcParams: map[string]float64{},
			},
			want: url.Values{
				"locate_version": []string{"v2"},
				"metro_rank":     []string{"0"},
				"index":          []string{"0"},
			},
		},
		{
			name:     "unmatched-host",
			hostname: "host",
			index:    0,
			p: paramOpts{
				version:   "v2",
				ranks:     map[string]int{"different-host": 0},
				svcParams: map[string]float64{},
			},
			want: url.Values{
				"locate_version": []string{"v2"},
				"index":          []string{"0"},
			},
		},
		{
			name:  "early-exit-true",
			index: 0,
			p: paramOpts{
				raw:     map[string][]string{static.EarlyExitParameter: {"250"}},
				version: "v2",
				svcParams: map[string]float64{
					static.EarlyExitParameter: 1,
				},
			},
			earlyExitProbability: 1,
			want: url.Values{
				static.EarlyExitParameter: []string{"250"},
				"locate_version":          []string{"v2"},
				"index":                   []string{"0"},
			},
		},
		{
			name:  "early-exit-false",
			index: 0,
			p: paramOpts{
				raw:       map[string][]string{static.EarlyExitParameter: {"250"}},
				version:   "v2",
				svcParams: map[string]float64{static.EarlyExitParameter: 0},
			},
			earlyExitProbability: 0,
			want: url.Values{
				"locate_version": []string{"v2"},
				"index":          []string{"0"},
			},
		},
		{
			name:  "max-cwnd-gain-and-early-exit-true",
			index: 0,
			p: paramOpts{
				raw: map[string][]string{
					static.EarlyExitParameter:   {"250"},
					static.MaxCwndGainParameter: {"512"},
				},
				version: "v2",
				svcParams: map[string]float64{
					static.EarlyExitParameter:   1,
					static.MaxCwndGainParameter: 1,
				},
			},
			earlyExitProbability: 1,
			want: url.Values{
				static.EarlyExitParameter:   []string{"250"},
				static.MaxCwndGainParameter: []string{"512"},
				"locate_version":            []string{"v2"},
				"index":                     []string{"0"},
			},
		},
		{
			name:  "max-cwnd-gain-and-max-elapsed-time-true",
			index: 0,
			p: paramOpts{
				raw: map[string][]string{
					static.MaxCwndGainParameter:    {"512"},
					static.MaxElapsedTimeParameter: {"5"},
				},
				version: "v2",
				svcParams: map[string]float64{
					static.MaxElapsedTimeParameter: 1,
					static.MaxCwndGainParameter:    1,
				},
			},
			earlyExitProbability: 1,
			want: url.Values{
				static.MaxCwndGainParameter:    []string{"512"},
				static.MaxElapsedTimeParameter: []string{"5"},
				"locate_version":               []string{"v2"},
				"index":                        []string{"0"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.client.extraParams(tt.hostname, tt.index, tt.p)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extraParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_limitRequest(t *testing.T) {
	tests := []struct {
		name   string
		limits limits.Agents
		t      time.Time
		req    *http.Request
		want   bool
	}{
		{
			name:   "allowed-user-agent-allowed-time",
			limits: limits.Agents{},
			t:      time.Now().UTC(),
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"foo"},
				},
			},
			want: false,
		},
		{
			name: "allowed-user-agent-limited-time",
			limits: limits.Agents{
				"foo": limits.NewCron("* * * * *", time.Minute), // Every minute of every hour.
			},
			t: time.Now().UTC(),
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"bar"},
				},
			},
			want: false,
		},
		{
			name: "limited-user-agent-allowed-time",
			limits: limits.Agents{
				"foo": limits.NewCron("*/30 * * * *", time.Minute), // Every 30th minute.
			},
			t: time.Date(2023, time.November, 16, 19, 29, 0, 0, time.UTC), // Request at minute 29.
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"foo"},
				},
			},
			want: false,
		},
		{
			name: "limited-user-agent-limited-time",
			limits: limits.Agents{
				"foo": limits.NewCron("*/30 * * * *", time.Minute), // Every 30th minute.
			},
			t: time.Date(2023, time.November, 16, 19, 30, 0, 0, time.UTC), // Request at minute 30.
			req: &http.Request{
				Header: http.Header{
					"User-Agent": []string{"foo"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				agentLimits: tt.limits,
			}
			if got := c.limitRequest(tt.t, tt.req); got != tt.want {
				t.Errorf("Client.limitRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRemoteAddr(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		remoteAddr    string
		expectedIP    string
	}{
		{
			name:          "single IP in X-Forwarded-For",
			xForwardedFor: "203.0.113.42",
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "two IPs in X-Forwarded-For (client + LB)",
			xForwardedFor: "203.0.113.42, 142.250.185.180",
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "four IPs in X-Forwarded-For (spoofed + client + LB)",
			xForwardedFor: "8.8.8.8, 1.1.1.1, 203.0.113.42, 142.250.185.180",
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "three IPs in X-Forwarded-For (spoofed + client + LB)",
			xForwardedFor: "8.8.8.8, 203.0.113.42, 142.250.185.180",
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "no X-Forwarded-For header (fallback to RemoteAddr)",
			xForwardedFor: "",
			remoteAddr:    "203.0.113.42:12345",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "no X-Forwarded-For header without port (fallback to RemoteAddr)",
			xForwardedFor: "",
			remoteAddr:    "203.0.113.42",
			expectedIP:    "203.0.113.42",
		},
		{
			name:          "IPs with spaces in X-Forwarded-For",
			xForwardedFor: "  8.8.8.8  ,  1.1.1.1  ,  203.0.113.42  ,  142.250.185.180  ",
			remoteAddr:    "192.168.1.1:12345",
			expectedIP:    "203.0.113.42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			req.RemoteAddr = tt.remoteAddr

			got := getRemoteAddr(req)
			if got != tt.expectedIP {
				t.Errorf("getRemoteAddr() = %v, want %v", got, tt.expectedIP)
			}
		})
	}
}
