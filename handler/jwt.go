package handler

import (
	"net/http"
)

// Verifier defines the interface for extracting JWT claims from HTTP requests.
// Different implementations support different verification modes.
type Verifier interface {
	// ExtractClaims extracts and optionally validates JWT claims from the request.
	// Returns the claims as a map, or an error if extraction/validation fails.
	ExtractClaims(req *http.Request) (map[string]interface{}, error)

	// Mode returns the name of the verification mode (for logging/debugging).
	Mode() string
}
