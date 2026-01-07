package jwtverifier

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

// ESPv1 extracts JWT claims from the X-Endpoint-API-UserInfo header
// set by Cloud Endpoints ESPv1 after JWT validation.
type ESPv1 struct{}

// NewESPv1 creates a new ESPv1 JWT verifier.
func NewESPv1() *ESPv1 {
	return &ESPv1{}
}

// ExtractClaims extracts JWT claims from the X-Endpoint-API-UserInfo header.
func (v *ESPv1) ExtractClaims(req *http.Request) (map[string]interface{}, error) {
	// Extract claims from the ESP header (trusted source after ESP validation)
	return v.extractFromESPHeader(req)
}

// extractFromESPHeader extracts claims from X-Endpoint-API-UserInfo header.
// This implements the ESPv1 format parsing logic.
func (v *ESPv1) extractFromESPHeader(req *http.Request) (map[string]interface{}, error) {
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

// Mode returns the verification mode name.
func (v *ESPv1) Mode() string {
	return "espv1"
}
