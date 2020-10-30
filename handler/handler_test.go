// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/static"
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
			wantStatus: http.StatusOK,
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
			c := NewClient(tt.project, tt.signer, tt.locator)

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

func TestClient_Heartbeat(t *testing.T) {
	tests := []struct {
		name    string
		Signer  Signer
		project string
		Locator Locator
	}{
		{
			// Provide basic coverage until handler implementation is complete.
			name: "placeholder",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.project, tt.Signer, tt.Locator)
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v2/heartbeat/ndt/ndt5", nil)
			c.Heartbeat(rw, req)
		})
	}
}

func TestNewClientDirect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		c := NewClientDirect("fake-project", nil, nil)
		if c == nil {
			t.Error("got nil client!")
		}
	})
}
