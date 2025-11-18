package jwtverifier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"

	log "github.com/sirupsen/logrus"
)

// DirectVerifier validates JWTs from the Authorization header using JWKS.
// This mode fetches the JWKS on every request (no caching) and validates
// the JWT signature. Intended for integration testing, not production use.
type DirectVerifier struct {
	jwksURL    *url.URL
	httpClient *http.Client
}

// NewDirectVerifier creates a new direct JWT verifier with JWKS validation.
func NewDirectVerifier(jwksURL *url.URL) (*DirectVerifier, error) {
	if jwksURL == nil {
		return nil, fmt.Errorf("JWKS URL cannot be nil")
	}

	if jwksURL.Scheme != "https" && jwksURL.Scheme != "http" {
		return nil, fmt.Errorf("JWKS URL must use http or https scheme, got: %s", jwksURL.Scheme)
	}

	return &DirectVerifier{
		jwksURL:    jwksURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// ExtractClaims extracts and validates JWT claims from the Authorization header.
// It fetches the JWKS on every request and validates the JWT signature. This
// is only meant to be used for local e2e testing.
// TODO: implement proper caching and JWKS reuse to allow non-GAE deployments.
func (v *DirectVerifier) ExtractClaims(req *http.Request) (map[string]interface{}, error) {
	// Extract Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("authorization header not found")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("authorization header must be in format: Bearer <token>")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Fetch JWKS
	jwks, err := v.fetchJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	// Parse JWT
	token, err := jwt.ParseSigned(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	// Verify JWT signature and extract claims
	claims, err := v.verifyAndExtractClaims(token, jwks)
	if err != nil {
		return nil, fmt.Errorf("JWT verification failed for JWKS %s: %w", v.jwksURL.String(), err)
	}

	log.WithFields(log.Fields{
		"mode": "direct",
	}).Debug("JWT verified successfully with JWKS")

	return claims, nil
}

// fetchJWKS fetches the JSON Web Key Set from the configured URL.
func (v *DirectVerifier) fetchJWKS() (*jose.JSONWebKeySet, error) {
	resp, err := v.httpClient.Get(v.jwksURL.String())
	if err != nil {
		return nil, fmt.Errorf("HTTP request to JWKS URL failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS JSON: %w", err)
	}

	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("JWKS contains no keys")
	}

	return &jwks, nil
}

// verifyAndExtractClaims verifies the JWT signature using keys from JWKS
// and extracts the claims.
func (v *DirectVerifier) verifyAndExtractClaims(token *jwt.JSONWebToken, jwks *jose.JSONWebKeySet) (map[string]interface{}, error) {
	var claims map[string]any
	var jwtClaims jwt.Claims
	var lastErr error

	// Try each key in the JWKS until one works
	for i, key := range jwks.Keys {
		err := token.Claims(key, &claims, &jwtClaims)
		if err == nil {
			// Signature verified successfully, now validate standard claims
			expectedClaims := jwt.Expected{
				Time: time.Now(),
			}

			if err := jwtClaims.Validate(expectedClaims); err != nil {
				return nil, fmt.Errorf("JWT claims validation failed: %w", err)
			}

			log.WithFields(log.Fields{
				"key_id":    key.KeyID,
				"key_index": i,
			}).Debug("JWT verified with JWKS key")

			return claims, nil
		}
		lastErr = err
	}

	// None of the keys worked
	return nil, fmt.Errorf("failed to verify JWT with any key in JWKS (tried %d keys): %w", len(jwks.Keys), lastErr)
}

// Mode returns the verification mode name.
func (v *DirectVerifier) Mode() string {
	return "direct"
}
