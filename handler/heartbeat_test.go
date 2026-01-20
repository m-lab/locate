package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m-lab/locate/auth/jwtverifier"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestClient_HeartbeatHandlers(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(*Client, http.ResponseWriter, *http.Request)
		method         string
		url            string
		headers        map[string]string
		wantStatusCode int
	}{
		{
			name:           "heartbeat_missing_upgrade_header",
			handler:        (*Client).Heartbeat,
			method:         http.MethodGet,
			url:            "/v2/heartbeat",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "heartbeat_jwt_missing_header",
			handler:        (*Client).HeartbeatJWT,
			method:         http.MethodGet,
			url:            "/v2/platform/heartbeat-jwt",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:    "heartbeat_jwt_invalid_header",
			handler: (*Client).HeartbeatJWT,
			method:  http.MethodGet,
			url:     "/v2/platform/heartbeat-jwt",
			headers: map[string]string{
				"X-Endpoint-API-UserInfo": "invalid-base64!",
			},
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.url, nil)

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			c := fakeClient(nil)
			tt.handler(c, rw, req)

			if rw.Code != tt.wantStatusCode {
				t.Errorf("%s wrong status code; got %d, want %d", tt.name, rw.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestClient_handleHeartbeats(t *testing.T) {
	wantErr := errors.New("connection error")
	tests := []struct {
		name    string
		ws      conn
		tracker heartbeat.StatusTracker
	}{
		{
			name: "read-err",
			ws: &fakeConn{
				err: wantErr,
			},
		},
		{
			name: "registration-err",
			ws: &fakeConn{
				msg: testdata.FakeRegistration,
			},
			tracker: &heartbeattest.FakeStatusTracker{Err: wantErr},
		},
		{
			name: "health-err",
			ws: &fakeConn{
				msg: testdata.FakeHealth,
			},
			tracker: &heartbeattest.FakeStatusTracker{Err: wantErr},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fakeClient(tt.tracker)
			err := c.handleHeartbeats(tt.ws, nil)
			if !errors.Is(err, wantErr) {
				t.Errorf("Client.handleHeartbeats() error = %v, wantErr %v", err, wantErr)
			}
		})
	}
}

func TestClient_extractJWTClaims(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		wantErr       bool
		wantOrgClaim  string
		errorContains string
	}{
		{
			name:          "missing_header",
			headerValue:   "",
			wantErr:       true,
			errorContains: "request must be processed through Cloud Endpoints",
		},
		{
			name:          "invalid_base64",
			headerValue:   "invalid-base64!",
			wantErr:       true,
			errorContains: "failed to decode X-Endpoint-API-UserInfo header",
		},
		{
			name:          "invalid_json",
			headerValue:   base64.StdEncoding.EncodeToString([]byte("not-json")),
			wantErr:       true,
			errorContains: "failed to parse X-Endpoint-API-UserInfo JSON",
		},
		{
			name:          "missing_claims_field",
			headerValue:   base64.StdEncoding.EncodeToString([]byte(`{"id":"user123","issuer":"test"}`)),
			wantErr:       true,
			errorContains: "claims field not found in X-Endpoint-API-UserInfo",
		},
		{
			name:          "invalid_claims_type",
			headerValue:   base64.StdEncoding.EncodeToString([]byte(`{"claims":123}`)),
			wantErr:       true,
			errorContains: "claims field is not a string",
		},
		{
			name:          "invalid_claims_json",
			headerValue:   base64.StdEncoding.EncodeToString([]byte(`{"claims":"invalid-json"}`)),
			wantErr:       true,
			errorContains: "failed to parse claims JSON string",
		},
		{
			name:         "valid_espv1_format",
			headerValue:  createValidESPv1Header("mlab-sandbox"),
			wantErr:      false,
			wantOrgClaim: "mlab-sandbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Endpoint-API-UserInfo", tt.headerValue)
			}

			c := fakeClient(nil)
			claims, err := c.extractJWTClaims(req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("extractJWTClaims() expected error but got none")
					return
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("extractJWTClaims() error = %v, want error containing %q", err, tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("extractJWTClaims() unexpected error = %v", err)
				return
			}

			if tt.wantOrgClaim != "" {
				orgClaim, ok := claims["org"]
				if !ok {
					t.Errorf("extractJWTClaims() missing org claim in result")
					return
				}
				if orgClaim != tt.wantOrgClaim {
					t.Errorf("extractJWTClaims() org claim = %v, want %v", orgClaim, tt.wantOrgClaim)
				}
			}
		})
	}
}

// createValidESPv1Header creates a valid X-Endpoint-API-UserInfo header value for testing
func createValidESPv1Header(org string) string {
	// ESPv1 format: claims field is a JSON string, not an object
	claims := map[string]interface{}{
		"iss": "token-exchange",
		"sub": "user123",
		"aud": "autojoin",
		"exp": 9999999999, // Far future
		"iat": 1600000000,
		"org": org,
	}
	claimsString, _ := json.Marshal(claims)

	espData := map[string]interface{}{
		"issuer":    "token-exchange",
		"audiences": []string{"autojoin"},
		"claims":    string(claimsString), // JSON string, not object
	}

	jsonBytes, _ := json.Marshal(espData)
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func fakeClient(t heartbeat.StatusTracker) *Client {
	locatorv2 := fakeLocatorV2{StatusTracker: t}
	// Use ESPv1 verifier for tests (it will work with the X-Endpoint-API-UserInfo header format used in tests)
	verifier := jwtverifier.NewESPv1()
	return NewClient("mlab-sandbox", &fakeSigner{}, &locatorv2,
		clientgeo.NewAppEngineLocator(), prom.NewAPI(nil), nil, nil, nil, nil, verifier)
}

type fakeConn struct {
	msg any
	err error
}

// ReadMessage returns 0, the JSON encoding of a fake message, and an error.
func (c *fakeConn) ReadMessage() (int, []byte, error) {
	jsonMsg, _ := json.Marshal(c.msg)
	return 0, jsonMsg, c.err
}

// SetReadDeadline returns nil.
func (c *fakeConn) SetReadDeadline(time.Time) error {
	return nil
}

// Close returns nil.
func (c *fakeConn) Close() error {
	return nil
}
