package jwtverifier

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestESPv1Verifier_ExtractClaims(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		authHeader  string
		wantErr     bool
		wantClaims  map[string]interface{}
	}{
		{
			name:        "valid ESP header",
			headerValue: createESPv1Header(map[string]interface{}{"org": "testorg", "tier": 1, "sub": "user123"}),
			wantErr:     false,
			wantClaims:  map[string]interface{}{"org": "testorg", "tier": float64(1), "sub": "user123"},
		},
		{
			name:        "missing header",
			headerValue: "",
			wantErr:     true,
		},
		{
			name:        "invalid base64",
			headerValue: "not-base64!@#$",
			wantErr:     true,
		},
		{
			name:        "invalid JSON",
			headerValue: base64.StdEncoding.EncodeToString([]byte("not json")),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := NewESPv1()

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Endpoint-API-UserInfo", tt.headerValue)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			claims, err := verifier.ExtractClaims(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractClaims() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if claims == nil {
					t.Fatal("Expected claims, got nil")
				}
				for key, expectedValue := range tt.wantClaims {
					if actualValue, ok := claims[key]; !ok {
						t.Errorf("Missing claim %s", key)
					} else if actualValue != expectedValue {
						t.Errorf("Claim %s = %v, want %v", key, actualValue, expectedValue)
					}
				}
			}
		})
	}
}

func TestESPv1Verifier_Mode(t *testing.T) {
	verifier := NewESPv1()
	if mode := verifier.Mode(); mode != "espv1" {
		t.Errorf("Mode() = %v, want espv1", mode)
	}
}

// createESPv1Header creates a properly formatted X-Endpoint-API-UserInfo header value
func createESPv1Header(claims map[string]interface{}) string {
	claimsJSON, _ := json.Marshal(claims)
	espData := map[string]interface{}{
		"claims": string(claimsJSON),
	}
	espJSON, _ := json.Marshal(espData)
	return base64.StdEncoding.EncodeToString(espJSON)
}
