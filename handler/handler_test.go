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
	"gopkg.in/square/go-jose.v2/jwt"
)

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
	err      error
	machines []string
}

func (l *fakeLocator) Nearest(ctx context.Context, service, lat, lon string) ([]string, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.machines, nil
}

func TestClient_TranslatedQuery(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		signer     Signer
		locator    *fakeLocator
		project    string
		latlon     string
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
			name:   "success-nearest-server",
			path:   "ndt/ndt5",
			signer: &fakeSigner{},
			locator: &fakeLocator{
				machines: []string{"mlab1-lga0t.measurement-lab.org"},
			},
			latlon:     "40.3,-70.4",
			wantKey:    "ws://:3001/ndt_protocol",
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.project, tt.signer, tt.locator)

			mux := http.NewServeMux()
			mux.HandleFunc("/v2/query/", c.TranslatedQuery)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+"/v2/query/"+tt.path, nil)
			rtx.Must(err, "Failed to create request")
			req.Header.Set("X-AppEngine-CityLatLong", tt.latlon)

			result := &v2.QueryResult{}
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
			if result.Error != nil && result.Error.Status != tt.wantStatus {
				t.Errorf("TranslatedQuery() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			if result.Error != nil {
				return
			}
			if result.Results == nil && tt.wantStatus == http.StatusOK {
				t.Errorf("TranslatedQuery() wrong status; got %d, want %d", result.Error.Status, tt.wantStatus)
			}
			//	pretty.Print(result)
			if len(tt.locator.machines) != len(result.Results) {
				t.Errorf("TranslateQuery() wrong result count; got %d, want %d",
					len(result.Results), len(tt.locator.machines))
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
