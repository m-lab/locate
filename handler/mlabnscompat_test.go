package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_mlabnsCompatNewInnerRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		url        string
		wantOK     bool
		wantStatus int
	}{
		{
			name:       "POST method is rejected",
			method:     http.MethodPost,
			url:        "/ndt",
			wantOK:     false,
			wantStatus: http.StatusNotImplemented,
		},
		{
			name:   "GET with no params succeeds",
			method: http.MethodGet,
			url:    "/ndt",
			wantOK: true,
		},
		{
			name:   "GET with format=json succeeds",
			method: http.MethodGet,
			url:    "/ndt?format=json",
			wantOK: true,
		},
		{
			name:       "GET with format=bt is rejected",
			method:     http.MethodGet,
			url:        "/ndt?format=bt",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "GET with policy=geo succeeds",
			method: http.MethodGet,
			url:    "/ndt?policy=geo",
			wantOK: true,
		},
		{
			name:   "GET with policy=metro and metro value succeeds",
			method: http.MethodGet,
			url:    "/ndt?policy=metro&metro=lga",
			wantOK: true,
		},
		{
			name:       "GET with policy=metro without metro value is rejected",
			method:     http.MethodGet,
			url:        "/ndt?policy=metro",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with policy=random is rejected",
			method:     http.MethodGet,
			url:        "/ndt?policy=random",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with policy=geo_options is rejected",
			method:     http.MethodGet,
			url:        "/ndt?policy=geo_options",
			wantOK:     false,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the outer request with a context to verify propagation.
			type ctxKey struct{}
			ctx := context.WithValue(context.Background(), ctxKey{}, "testvalue")
			outerReq := httptest.NewRequest(tt.method, tt.url, http.NoBody)
			outerReq = outerReq.WithContext(ctx)
			outerReq.Header.Set("X-Forwarded-For", "1.2.3.4")

			rw := httptest.NewRecorder()
			innerReq, ok := mlabnsCompatNewInnerRequest(rw, outerReq)

			assert.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				assert.Nil(t, innerReq)
				assert.Equal(t, tt.wantStatus, rw.Code)
				return
			}

			require.NotNil(t, innerReq)
			assert.Equal(t, http.MethodGet, innerReq.Method)
			assert.Equal(t, "/v2/nearest/ndt/ndt7", innerReq.URL.Path)
			assert.Equal(t, "1.2.3.4", innerReq.Header.Get("X-Forwarded-For"))
			assert.Equal(t, "testvalue", innerReq.Context().Value(ctxKey{}))
		})
	}
}

// newV2ResponseRecorder creates an [*httptest.ResponseRecorder] that simulates
// a response from the [*Client.Nearest] handler.
func newV2ResponseRecorder(code int, body any) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rec.Code = code
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		rec.Body.Write(data)
	}
	return rec
}

func Test_mlabnsCompatNewResponse(t *testing.T) {
	// Valid v2 result used by the success case and as a template for partial-failure cases.
	validResult := v2.NearestResult{
		Results: []v2.Target{
			{
				Machine:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
				Hostname: "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org",
				Location: &v2.Location{
					City:    "New York",
					Country: "US",
				},
			},
		},
	}

	tests := []struct {
		name       string
		recorder   *httptest.ResponseRecorder
		wantOK     bool
		wantStatus int
		wantResp   *mlabnsCompatResponse
	}{
		{
			name:       "inner 500 maps to 502",
			recorder:   newV2ResponseRecorder(http.StatusInternalServerError, nil),
			wantOK:     false,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "inner 429 maps to 502",
			recorder:   newV2ResponseRecorder(http.StatusTooManyRequests, nil),
			wantOK:     false,
			wantStatus: http.StatusBadGateway,
		},
		{
			name: "invalid JSON body maps to 502",
			recorder: func() *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				rec.Code = http.StatusOK
				rec.Body.WriteString("{garbage")
				return rec
			}(),
			wantOK:     false,
			wantStatus: http.StatusBadGateway,
		},
		{
			name:       "empty results array maps to 204",
			recorder:   newV2ResponseRecorder(http.StatusOK, v2.NearestResult{}),
			wantOK:     false,
			wantStatus: http.StatusNoContent,
		},
		{
			name: "nil location maps to 204",
			recorder: newV2ResponseRecorder(http.StatusOK, v2.NearestResult{
				Results: []v2.Target{
					{
						Machine:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
						Hostname: "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org",
						Location: nil,
					},
				},
			}),
			wantOK:     false,
			wantStatus: http.StatusNoContent,
		},
		{
			name: "unparseable machine maps to 204",
			recorder: newV2ResponseRecorder(http.StatusOK, v2.NearestResult{
				Results: []v2.Target{
					{
						Machine:  "not-a-valid-hostname",
						Hostname: "not-a-valid-hostname",
						Location: &v2.Location{
							City:    "New York",
							Country: "US",
						},
					},
				},
			}),
			wantOK:     false,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "valid response produces correct compat output",
			recorder:   newV2ResponseRecorder(http.StatusOK, validResult),
			wantOK:     true,
			wantStatus: http.StatusOK,
			wantResp: &mlabnsCompatResponse{
				City:    "New York",
				Country: "US",
				FQDN:    "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org",
				IP:      []string{"127.0.0.1", "::1"},
				Site:    "lga0t",
				URL:     "https://ndt-mlab1-lga0t.mlab-oti.measurement-lab.org/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			resp, ok := mlabnsCompatNewResponse(rw, tt.recorder)

			assert.Equal(t, tt.wantOK, ok)

			if !tt.wantOK {
				assert.Nil(t, resp)
				assert.Equal(t, tt.wantStatus, rw.Code)
				return
			}

			require.NotNil(t, resp)
			assert.Equal(t, tt.wantResp, resp)
		})
	}
}

func Test_mlabnsCompatSerializeAndSendResponse(t *testing.T) {
	validResult := v2.NearestResult{
		Results: []v2.Target{
			{
				Machine:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
				Hostname: "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org",
				Location: &v2.Location{
					City:    "New York",
					Country: "US",
				},
			},
		},
	}

	tests := []struct {
		name       string
		recorder   *httptest.ResponseRecorder
		wantStatus int
		wantBody   bool
	}{
		{
			name:       "inner failure returns 502 with no body",
			recorder:   newV2ResponseRecorder(http.StatusInternalServerError, nil),
			wantStatus: http.StatusBadGateway,
			wantBody:   false,
		},
		{
			name:       "valid inner response returns 200 with JSON body",
			recorder:   newV2ResponseRecorder(http.StatusOK, validResult),
			wantStatus: http.StatusOK,
			wantBody:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			mlabnsCompatSerializeAndSendResponse(rw, tt.recorder)

			assert.Equal(t, tt.wantStatus, rw.Code)

			if !tt.wantBody {
				assert.Empty(t, rw.Body.String())
				return
			}

			assert.Equal(t, "application/json", rw.Header().Get("content-type"))

			var got mlabnsCompatResponse
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &got))
			assert.Equal(t, "New York", got.City)
			assert.Equal(t, "lga0t", got.Site)
			assert.Equal(t, "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org", got.FQDN)
		})
	}
}

func TestClient_MLabNSCompat(t *testing.T) {
	validLocator := &fakeLocatorV2{
		targets: []v2.Target{
			{
				Machine:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
				Hostname: "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org",
				Location: &v2.Location{City: "New York", Country: "US"},
			},
		},
		urls: []url.URL{
			{Scheme: "wss", Host: ":3010", Path: "/ndt/v7/download"},
		},
	}

	tests := []struct {
		name       string
		path       string
		locator    *fakeLocatorV2
		wantStatus int
	}{
		{
			name:       "success end-to-end",
			path:       "/ndt?format=json",
			locator:    validLocator,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid option returns 400",
			path:       "/ndt?format=bt",
			locator:    validLocator,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "locator error propagates as 502",
			path: "/ndt?format=json",
			locator: &fakeLocatorV2{
				err: errors.New("no healthy servers"),
			},
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(
				"mlab-testing",
				&fakeSigner{},
				tt.locator,
				clientgeo.NewAppEngineLocator(),
				prom.NewAPI(nil),
				nil, nil, nil, nil, nil,
			)

			mux := http.NewServeMux()
			mux.HandleFunc("/ndt", c.MLabNSCompat)
			srv := httptest.NewServer(mux)
			defer srv.Close()

			req, err := http.NewRequest(http.MethodGet, srv.URL+tt.path, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("X-AppEngine-CityLatLong", "40.3,-70.4")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantStatus != http.StatusOK {
				return
			}

			var got mlabnsCompatResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
			assert.Equal(t, "New York", got.City)
			assert.Equal(t, "US", got.Country)
			assert.Equal(t, "ndt-mlab1-lga0t.mlab-oti.measurement-lab.org", got.FQDN)
			assert.Equal(t, "lga0t", got.Site)
			assert.Equal(t, []string{"127.0.0.1", "::1"}, got.IP)
		})
	}
}
