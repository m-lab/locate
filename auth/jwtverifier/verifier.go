// Package jwtverifier provides JWT claim extraction and verification backends.
//
// The package supports three modes:
//   - ESPv1: Extract claims from X-Endpoint-API-UserInfo header set by Cloud Endpoints
//   - Direct: Validate JWT from Authorization header using JWKS
//   - Insecure: Parse JWT without validation (development/testing only)
package jwtverifier
