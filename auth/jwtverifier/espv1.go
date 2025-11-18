package jwtverifier

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"gopkg.in/square/go-jose.v2/jwt"

	log "github.com/sirupsen/logrus"
)

// ESPv1Verifier extracts JWT claims from the X-Endpoint-API-UserInfo header
// set by Cloud Endpoints ESPv1 after JWT validation.
//
// Defense-in-depth: This verifier also checks the Authorization header (if present)
// and logs a security warning if the claims don't match the ESP header.
type ESPv1Verifier struct{}

// NewESPv1Verifier creates a new ESPv1 JWT verifier.
func NewESPv1Verifier() *ESPv1Verifier {
	return &ESPv1Verifier{}
}

// ExtractClaims extracts JWT claims from the X-Endpoint-API-UserInfo header.
// It also performs defense-in-depth checking against the Authorization header.
func (v *ESPv1Verifier) ExtractClaims(req *http.Request) (map[string]interface{}, error) {
	// Extract claims from the ESP header (trusted source after ESP validation)
	espClaims, err := v.extractFromESPHeader(req)
	if err != nil {
		return nil, err
	}

	// Defense-in-depth: Verify Authorization header matches (if present)
	v.verifyAuthorizationHeader(req, espClaims)

	return espClaims, nil
}

// Mode returns the verification mode name.
func (v *ESPv1Verifier) Mode() string {
	return "espv1"
}

// extractFromESPHeader extracts claims from X-Endpoint-API-UserInfo header.
// This implements the ESPv1 format parsing logic.
func (v *ESPv1Verifier) extractFromESPHeader(req *http.Request) (map[string]interface{}, error) {
	// Get the X-Endpoint-API-UserInfo header set by Cloud Endpoints
	userInfoHeader := req.Header.Get("X-Endpoint-API-UserInfo")
	if userInfoHeader == "" {
		return nil, fmt.Errorf("request must be processed through Cloud Endpoints: X-Endpoint-API-UserInfo header not found")
	}

	// Decode the base64-encoded header content (Cloud Endpoints uses standard base64)
	decoded, err := base64.StdEncoding.DecodeString(userInfoHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode X-Endpoint-API-UserInfo header: %w", err)
	}

	// Parse the JSON content
	var espData map[string]interface{}
	if err := json.Unmarshal(decoded, &espData); err != nil {
		return nil, fmt.Errorf("failed to parse X-Endpoint-API-UserInfo JSON: %w", err)
	}

	// Extract the claims field from ESPv1 format
	claimsInterface, ok := espData["claims"]
	if !ok {
		return nil, fmt.Errorf("claims field not found in X-Endpoint-API-UserInfo")
	}

	// In ESPv1, the claims field is a JSON string, not an object
	claimsString, ok := claimsInterface.(string)
	if !ok {
		return nil, fmt.Errorf("claims field is not a string")
	}

	// Parse the claims JSON string
	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(claimsString), &claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims JSON string: %w", err)
	}

	return claims, nil
}

// verifyAuthorizationHeader performs defense-in-depth checking by comparing
// the Authorization header JWT claims with the ESP header claims.
func (v *ESPv1Verifier) verifyAuthorizationHeader(req *http.Request, espClaims map[string]interface{}) {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		// No Authorization header present - this is acceptable
		return
	}

	// Extract Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.WithFields(log.Fields{
			"mode": "espv1",
		}).Warn("SECURITY: Authorization header present but not in Bearer format")
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse JWT without verification to extract claims
	authClaims, err := v.parseJWTWithoutVerification(tokenString)
	if err != nil {
		log.WithFields(log.Fields{
			"mode":  "espv1",
			"error": err.Error(),
		}).Warn("SECURITY: Failed to parse Authorization header JWT for defense-in-depth check")
		return
	}

	// Compare critical claims
	if !v.claimsMatch(espClaims, authClaims) {
		log.WithFields(log.Fields{
			"mode":       "espv1",
			"esp_claims": espClaims,
			"auth_claims": authClaims,
		}).Warn("SECURITY: Authorization header claims don't match X-Endpoint-API-UserInfo claims")
	}
}

// parseJWTWithoutVerification parses a JWT and extracts claims without signature verification.
// This is used only for defense-in-depth comparison, not for trusting the JWT.
func (v *ESPv1Verifier) parseJWTWithoutVerification(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.ParseSigned(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	var claims map[string]interface{}
	if err := token.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	return claims, nil
}

// claimsMatch compares critical fields between ESP claims and Authorization header claims.
// Returns true if key fields match, false otherwise.
func (v *ESPv1Verifier) claimsMatch(espClaims, authClaims map[string]interface{}) bool {
	// Compare critical claim fields
	criticalFields := []string{"sub", "iss", "exp", "org", "tier"}

	for _, field := range criticalFields {
		espValue, espHas := espClaims[field]
		authValue, authHas := authClaims[field]

		// If field exists in both, values must match
		if espHas && authHas {
			if !reflect.DeepEqual(espValue, authValue) {
				return false
			}
		}
		// If field exists in only one, that's also a mismatch
		if espHas != authHas {
			return false
		}
	}

	return true
}
