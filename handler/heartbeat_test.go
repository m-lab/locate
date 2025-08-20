package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestClient_Heartbeat_Error(t *testing.T) {
	rw := httptest.NewRecorder()
	// The header from this request will not contain the
	// necessary "upgrade" tokens.
	req := httptest.NewRequest(http.MethodGet, "/v2/heartbeat", nil)
	c := fakeClient(nil)
	c.Heartbeat(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("Heartbeat() wrong status code; got %d, want %d", rw.Code, http.StatusBadRequest)
	}
}

func TestClient_HeartbeatJWT_MissingHeader(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/platform/heartbeat-jwt", nil)
	c := fakeClient(nil)
	c.HeartbeatJWT(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Errorf("HeartbeatJWT() without X-Endpoint-API-UserInfo should return 401, got %d", rw.Code)
	}
}

func TestClient_HeartbeatJWT_InvalidHeader(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/platform/heartbeat-jwt", nil)
	req.Header.Set("X-Endpoint-API-UserInfo", "invalid-base64!")
	c := fakeClient(nil)
	c.HeartbeatJWT(rw, req)

	if rw.Code != http.StatusUnauthorized {
		t.Errorf("HeartbeatJWT() with invalid header should return 401, got %d", rw.Code)
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
		name           string
		headerValue    string
		wantErr        bool
		wantOrgClaim   string
		errorContains  string
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
			headerValue:   base64.RawURLEncoding.EncodeToString([]byte("not-json")),
			wantErr:       true,
			errorContains: "failed to parse X-Endpoint-API-UserInfo JSON",
		},
		{
			name: "missing_claims_field",
			headerValue: base64.RawURLEncoding.EncodeToString([]byte(`{"id":"user123","issuer":"test"}`)),
			wantErr:       true,
			errorContains: "claims field not found in X-Endpoint-API-UserInfo",
		},
		{
			name: "invalid_claims_type",
			headerValue: base64.RawURLEncoding.EncodeToString([]byte(`{"claims":"not-an-object"}`)),
			wantErr:       true,
			errorContains: "claims field is not a valid JSON object",
		},
		{
			name: "valid_espv1_format",
			headerValue: createValidESPv1Header("mlab-sandbox"),
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
	espData := map[string]interface{}{
		"id":       "user123",
		"issuer":   "token-exchange",
		"email":    "test@example.com",
		"audiences": []string{"autojoin"},
		"claims": map[string]interface{}{
			"iss": "token-exchange",
			"sub": "user123",
			"aud": "autojoin",
			"exp": 9999999999, // Far future
			"iat": 1600000000,
			"org": org,
		},
	}

	jsonBytes, _ := json.Marshal(espData)
	return base64.RawURLEncoding.EncodeToString(jsonBytes)
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
	return NewClient("mlab-sandbox", &fakeSigner{}, &locatorv2,
		clientgeo.NewAppEngineLocator(), prom.NewAPI(nil), nil, nil, nil)
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
