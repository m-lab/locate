// Package jwtverifier provides JWT claim extraction and verification backends.
//
// The package supports three modes:
//   - ESPv1: Extract claims from X-Endpoint-API-UserInfo header set by Cloud Endpoints
//   - Direct: Validate JWT from Authorization header using JWKS
//   - Insecure: Parse JWT without validation (development/testing only)
package jwtverifier

import (
	"net/http"
)

// JWTVerifier defines the interface for extracting JWT claims from HTTP requests.
// Different implementations support different verification modes.
type JWTVerifier interface {
	// ExtractClaims extracts and optionally validates JWT claims from the request.
	// Returns the claims as a map, or an error if extraction/validation fails.
	ExtractClaims(req *http.Request) (map[string]interface{}, error)

	// Mode returns the name of the verification mode (for logging/debugging).
	Mode() string
}
