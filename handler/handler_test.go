// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/auth/jwtverifier"
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

// createESPv1HeaderWithTier creates a valid X-Endpoint-API-UserInfo header value for testing
// with org and tier claims.
func createESPv1HeaderWithTier(org string, tier interface{}) string {
	claims := map[string]interface{}{
		"iss": "token-exchange",
		"sub": "user123",
		"aud": "autojoin",
		"exp": 9999999999,
		"iat": 1600000000,
		"org": org,
	}
	if tier != nil {
		claims["tier"] = tier
	}
	claimsString, _ := json.Marshal(claims)

	espData := map[string]interface{}{
		"issuer":    "token-exchange",
		"audiences": []string{"autojoin"},
		"claims":    string(claimsString),
	}

	jsonBytes, _ := json.Marshal(espData)
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// createESPv1HeaderWithoutOrg creates a header without the org claim.
func createESPv1HeaderWithoutOrg() string {
	claims := map[string]interface{}{
		"iss":  "token-exchange",
		"sub":  "user123",
		"aud":  "autojoin",
		"exp":  9999999999,
		"iat":  1600000000,
		"tier": 1,
	}
	claimsString, _ := json.Marshal(claims)

	espData := map[string]interface{}{
		"issuer":    "token-exchange",
		"audiences": []string{"autojoin"},
		"claims":    string(claimsString),
	}

	jsonBytes, _ := json.Marshal(espData)
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

func TestClient_PriorityNearest(t *testing.T) {
	// Start miniredis for rate limiter tests
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", s.Addr())
		},
	}

	cleanRedis := func() {
		conn := pool.Get()
		defer conn.Close()
		conn.Do("FLUSHDB")
	}

	tierLimits := limits.TierLimits{
		1: limits.LimitConfig{Interval: time.Hour, MaxEvents: 100},
		2: limits.LimitConfig{Interval: time.Hour, MaxEvents: 2}, // Low limit for testing
	}

	tests := []struct {
		name           string
		path           string
		signer         Signer
		locator        *fakeLocatorV2
		cl             ClientLocator
		tierLimits     limits.TierLimits
		ipLimiter      Limiter
		header         http.Header
		setupRedis     func()
		wantStatus     int
		wantFallback   bool // If true, expects fallback to regular Nearest behavior
	}{
		{
			name:   "success-with-valid-jwt-and-tier",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			ipLimiter: limits.NewRateLimiter(pool, limits.RateLimitConfig{
				IPConfig:   limits.LimitConfig{Interval: time.Hour, MaxEvents: 60},
				IPUAConfig: limits.LimitConfig{Interval: time.Hour, MaxEvents: 30},
				KeyPrefix:  "test:",
			}),
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyX", 1)},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "fallback-missing-jwt-header",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			header: http.Header{
				"X-AppEngine-CityLatLong": []string{"40.3,-70.4"},
				"X-Forwarded-For":         []string{"192.0.2.1"},
				// No X-Endpoint-API-UserInfo header
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "fallback-missing-org-claim",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithoutOrg()},
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "fallback-missing-tier-claim",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyX", nil)}, // No tier
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "fallback-invalid-tier-format",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyX", "not-a-number")},
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "fallback-unconfigured-tier",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyX", 99)}, // Tier 99 not configured
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "fallback-limiter-not-supporting-tier",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			ipLimiter: &fakeRateLimiter{ // fakeRateLimiter doesn't support tier-based limiting
				status: limits.LimitStatus{IsLimited: false},
			},
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.1"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyX", 1)},
			},
			wantStatus:   http.StatusOK,
			wantFallback: true,
		},
		{
			name:   "rate-limit-exceeded",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			ipLimiter: limits.NewRateLimiter(pool, limits.RateLimitConfig{
				IPConfig:   limits.LimitConfig{Interval: time.Hour, MaxEvents: 60},
				IPUAConfig: limits.LimitConfig{Interval: time.Hour, MaxEvents: 30},
				KeyPrefix:  "test:",
			}),
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.100"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("ratelimited-org", 2)}, // Tier 2 has max 2 events
			},
			setupRedis: func() {
				// Pre-populate Redis to simulate hitting the rate limit
				rl := limits.NewRateLimiter(pool, limits.RateLimitConfig{
					IPConfig:   limits.LimitConfig{Interval: time.Hour, MaxEvents: 60},
					IPUAConfig: limits.LimitConfig{Interval: time.Hour, MaxEvents: 30},
					KeyPrefix:  "test:",
				})
				// Make 2 requests to hit the limit (tier 2 max is 2)
				tierConfig := limits.LimitConfig{Interval: time.Hour, MaxEvents: 2}
				rl.IsLimitedWithTier("ratelimited-org", "192.0.2.100", tierConfig)
				rl.IsLimitedWithTier("ratelimited-org", "192.0.2.100", tierConfig)
			},
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:   "success-tier-as-float64",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			ipLimiter: limits.NewRateLimiter(pool, limits.RateLimitConfig{
				IPConfig:   limits.LimitConfig{Interval: time.Hour, MaxEvents: 60},
				IPUAConfig: limits.LimitConfig{Interval: time.Hour, MaxEvents: 30},
				KeyPrefix:  "test:",
			}),
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.50"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyY", 1.0)}, // float64 tier
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "success-tier-as-string",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
				urls: []url.URL{
					{Scheme: "ws", Host: ":3001", Path: "/ndt_protocol"},
				},
			},
			tierLimits: tierLimits,
			ipLimiter: limits.NewRateLimiter(pool, limits.RateLimitConfig{
				IPConfig:   limits.LimitConfig{Interval: time.Hour, MaxEvents: 60},
				IPUAConfig: limits.LimitConfig{Interval: time.Hour, MaxEvents: 30},
				KeyPrefix:  "test:",
			}),
			header: http.Header{
				"X-AppEngine-CityLatLong":  []string{"40.3,-70.4"},
				"X-Forwarded-For":          []string{"192.0.2.51"},
				"X-Endpoint-API-UserInfo":  []string{createESPv1HeaderWithTier("companyZ", "1")}, // string tier
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanRedis()
			if tt.setupRedis != nil {
				tt.setupRedis()
			}

			if tt.cl == nil {
				tt.cl = clientgeo.NewAppEngineLocator()
			}

			verifier := jwtverifier.NewESPv1()
			c := NewClient("test-project", tt.signer, tt.locator, tt.cl, prom.NewAPI(nil),
				nil, tt.tierLimits, tt.ipLimiter, nil, verifier)

			mux := http.NewServeMux()
			mux.HandleFunc("/v2/priority/nearest/", c.PriorityNearest)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/priority/nearest/"+tt.path, nil)
			rtx.Must(err, "Failed to create request")
			req.Header = tt.header

			result := &v2.NearestResult{}
			resp, err := proxy.UnmarshalResponse(req, result)
			if err != nil {
				t.Fatalf("Failed to get response: %v", err)
			}

			// Check status code
			if result.Error != nil {
				if result.Error.Status != tt.wantStatus {
					t.Errorf("PriorityNearest() status = %d, want %d", result.Error.Status, tt.wantStatus)
				}
			} else if tt.wantStatus != http.StatusOK {
				t.Errorf("PriorityNearest() expected error status %d, got success", tt.wantStatus)
			}

			// Verify CORS headers are set
			if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
				t.Errorf("PriorityNearest() wrong Access-Control-Allow-Origin header; got %s, want '*'",
					resp.Header.Get("Access-Control-Allow-Origin"))
			}

			// For successful requests, verify we got results
			if tt.wantStatus == http.StatusOK && len(result.Results) == 0 {
				t.Errorf("PriorityNearest() expected results but got none")
			}
		})
	}
}
