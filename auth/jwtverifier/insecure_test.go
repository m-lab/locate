package jwtverifier

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gopkg.in/square/go-jose.v2"
)

func TestInsecureVerifier_ExtractClaims(t *testing.T) {
	// Set environment variable for insecure mode
	os.Setenv("ALLOW_INSECURE_JWT", "true")
	defer os.Unsetenv("ALLOW_INSECURE_JWT")

	verifier, err := NewInsecure()
	if err != nil {
		t.Fatalf("Failed to create insecure verifier: %v", err)
	}

	tests := []struct {
		name       string
		createReq  func() *http.Request
		wantErr    bool
		wantClaims map[string]interface{}
	}{
		{
			name: "valid unsigned JWT",
			createReq: func() *http.Request {
				// Create an unsigned JWT (just for parsing, not for security)
				token := createUnsignedJWT(t, map[string]interface{}{"org": "testorg", "tier": 2})
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			wantErr:    false,
			wantClaims: map[string]interface{}{"org": "testorg", "tier": float64(2)},
		},
		{
			name: "missing authorization header",
			createReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.createReq()
			claims, err := verifier.ExtractClaims(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractClaims() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
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

func TestNewInsecure_RequiresEnvVar(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("ALLOW_INSECURE_JWT")

	verifier, err := NewInsecure()
	if err == nil {
		t.Error("Expected error when ALLOW_INSECURE_JWT is not set")
	}
	if verifier != nil {
		t.Error("Expected nil verifier when env var not set")
	}
}

func TestInsecureVerifier_Mode(t *testing.T) {
	os.Setenv("ALLOW_INSECURE_JWT", "true")
	defer os.Unsetenv("ALLOW_INSECURE_JWT")

	verifier, _ := NewInsecure()
	if mode := verifier.Mode(); mode != "insecure" {
		t.Errorf("Mode() = %v, want insecure", mode)
	}
}

// createUnsignedJWT creates an unsigned JWT for insecure mode testing
func createUnsignedJWT(t *testing.T, claims map[string]interface{}) string {
	// Generate a key just for signing (will be verified insecurely anyway)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	signingKey := jose.SigningKey{Algorithm: jose.ES256, Key: privateKey}
	return createSignedJWT(t, signingKey, claims)
}
