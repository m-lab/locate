package jwtverifier

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// Test ESPv1Verifier
func TestESPv1Verifier_ExtractClaims(t *testing.T) {
	tests := []struct {
		name          string
		headerValue   string
		authHeader    string
		wantErr       bool
		wantClaims    map[string]interface{}
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
			verifier := NewESPv1Verifier()

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
	verifier := NewESPv1Verifier()
	if mode := verifier.Mode(); mode != "espv1" {
		t.Errorf("Mode() = %v, want espv1", mode)
	}
}

// Test DirectVerifier
func TestDirectVerifier_ExtractClaims(t *testing.T) {
	// Create a test JWKS server
	testKey, jwks := createTestJWKS(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	verifier, err := NewDirectVerifier(serverURL)
	if err != nil {
		t.Fatalf("Failed to create verifier: %v", err)
	}

	tests := []struct {
		name       string
		createReq  func() *http.Request
		wantErr    bool
		wantClaims map[string]interface{}
	}{
		{
			name: "valid JWT",
			createReq: func() *http.Request {
				token := createSignedJWT(t, testKey, map[string]interface{}{"org": "testorg", "tier": 1})
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			wantErr:    false,
			wantClaims: map[string]interface{}{"org": "testorg", "tier": float64(1)},
		},
		{
			name: "missing authorization header",
			createReq: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			wantErr: true,
		},
		{
			name: "invalid bearer format",
			createReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "InvalidFormat token")
				return req
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

func TestNewDirectVerifier(t *testing.T) {
	tests := []struct {
		name      string
		jwksURL   string
		wantErr   bool
		parseErr  bool // If true, URL parsing itself should fail
	}{
		{
			name:    "valid HTTPS URL",
			jwksURL: "https://example.com/.well-known/jwks.json",
			wantErr: false,
		},
		{
			name:    "valid HTTP URL (with warning)",
			jwksURL: "http://localhost:8080/jwks",
			wantErr: false,
		},
		{
			name:     "invalid URL",
			jwksURL:  "://invalid",
			parseErr: true,
		},
		{
			name:    "invalid scheme",
			jwksURL: "ftp://example.com/jwks",
			wantErr: true,
		},
		{
			name:    "nil URL",
			jwksURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var parsedURL *url.URL
			var parseErr error

			if tt.jwksURL != "" {
				parsedURL, parseErr = url.Parse(tt.jwksURL)
				if tt.parseErr {
					if parseErr == nil {
						t.Error("Expected URL parse error, got nil")
					}
					return
				}
				if parseErr != nil {
					t.Fatalf("Unexpected URL parse error: %v", parseErr)
				}
			}

			verifier, err := NewDirectVerifier(parsedURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDirectVerifier() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && verifier == nil {
				t.Error("Expected non-nil verifier")
			}
		})
	}
}

func TestDirectVerifier_Mode(t *testing.T) {
	testURL, _ := url.Parse("https://example.com/jwks")
	verifier, _ := NewDirectVerifier(testURL)
	if mode := verifier.Mode(); mode != "direct" {
		t.Errorf("Mode() = %v, want direct", mode)
	}
}

// Test InsecureVerifier
func TestInsecureVerifier_ExtractClaims(t *testing.T) {
	// Set environment variable for insecure mode
	os.Setenv("ALLOW_INSECURE_JWT", "true")
	defer os.Unsetenv("ALLOW_INSECURE_JWT")

	verifier, err := NewInsecureVerifier()
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

func TestNewInsecureVerifier_RequiresEnvVar(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv("ALLOW_INSECURE_JWT")

	verifier, err := NewInsecureVerifier()
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

	verifier, _ := NewInsecureVerifier()
	if mode := verifier.Mode(); mode != "insecure" {
		t.Errorf("Mode() = %v, want insecure", mode)
	}
}

// Helper functions

// createESPv1Header creates a properly formatted X-Endpoint-API-UserInfo header value
func createESPv1Header(claims map[string]interface{}) string {
	claimsJSON, _ := json.Marshal(claims)
	espData := map[string]interface{}{
		"claims": string(claimsJSON),
	}
	espJSON, _ := json.Marshal(espData)
	return base64.StdEncoding.EncodeToString(espJSON)
}

// createTestJWKS creates a test signing key and JWKS for testing
func createTestJWKS(t *testing.T) (jose.SigningKey, jose.JSONWebKeySet) {
	// Generate an ECDSA private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create JWKS with the public key
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &privateKey.PublicKey,
				KeyID:     "test-key",
				Algorithm: string(jose.ES256),
				Use:       "sig",
			},
		},
	}

	return jose.SigningKey{Algorithm: jose.ES256, Key: privateKey}, jwks
}

// createSignedJWT creates a signed JWT with the given claims
func createSignedJWT(t *testing.T, signingKey jose.SigningKey, claims map[string]interface{}) string {
	signer, err := jose.NewSigner(signingKey, nil)
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	builder := jwt.Signed(signer)
	builder = builder.Claims(claims)

	token, err := builder.CompactSerialize()
	if err != nil {
		t.Fatalf("Failed to create JWT: %v", err)
	}

	return token
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
