package jwtverifier

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"gopkg.in/square/go-jose.v2/jwt"

	log "github.com/sirupsen/logrus"
)

// InsecureVerifier parses JWTs from the Authorization header WITHOUT signature verification.
// This mode is ONLY for development and testing. It requires the ALLOW_INSECURE_JWT=true
// environment variable to be set as a safety check.
//
// WARNING: Never use this in production - it accepts any JWT regardless of signature.
type InsecureVerifier struct {
	warnedOnce sync.Once
}

// NewInsecureVerifier creates a new insecure JWT verifier.
// Returns an error if the ALLOW_INSECURE_JWT environment variable is not set to "true".
func NewInsecureVerifier() (*InsecureVerifier, error) {
	// Require explicit environment variable as safety check
	if os.Getenv("ALLOW_INSECURE_JWT") != "true" {
		return nil, fmt.Errorf("insecure JWT mode requires ALLOW_INSECURE_JWT=true environment variable")
	}

	log.Warn("======================================================================")
	log.Warn("INSECURE JWT MODE ENABLED - JWTs will NOT be validated!")
	log.Warn("This mode should ONLY be used in development/testing environments")
	log.Warn("DO NOT USE IN PRODUCTION")
	log.Warn("======================================================================")

	return &InsecureVerifier{}, nil
}

// ExtractClaims extracts JWT claims from the Authorization header without signature verification.
func (v *InsecureVerifier) ExtractClaims(req *http.Request) (map[string]interface{}, error) {
	// Log warning on first use (per-verifier instance)
	v.warnedOnce.Do(func() {
		log.Warn("INSECURE MODE: Parsing JWT without signature verification")
	})

	// Extract Authorization header
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("Authorization header not found")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("Authorization header must be in format: Bearer <token>")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse JWT without verification
	token, err := jwt.ParseSigned(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	// Extract claims WITHOUT signature verification
	var claims map[string]interface{}
	if err := token.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	log.WithFields(log.Fields{
		"mode":   "insecure",
		"claims": claims,
	}).Debug("JWT claims extracted (UNVERIFIED)")

	return claims, nil
}

// Mode returns the verification mode name.
func (v *InsecureVerifier) Mode() string {
	return "insecure"
}
