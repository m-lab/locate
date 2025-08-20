package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
	log "github.com/sirupsen/logrus"
)

var readDeadline = static.WebsocketReadDeadline

type conn interface {
	ReadMessage() (int, []byte, error)
	SetReadDeadline(time.Time) error
	Close() error
}

// Heartbeat implements /v2/platform/heartbeat requests.
// It starts a new persistent connection and a new goroutine
// to read incoming messages. This endpoint is protected by API key authentication.
func (c *Client) Heartbeat(rw http.ResponseWriter, req *http.Request) {
	c.heartbeatHandler(rw, req, nil)
}

// HeartbeatJWT implements /v2/platform/heartbeat-jwt requests.
// It starts a new persistent connection and a new goroutine
// to read incoming messages. This endpoint is protected by JWT authentication
// with organization validation.
func (c *Client) HeartbeatJWT(rw http.ResponseWriter, req *http.Request) {
	// Extract the JWT token from the request context
	claims, err := c.extractJWTClaims(req)
	if err != nil {
		log.Errorf("failed to extract JWT claims: %v", err)
		http.Error(rw, "Unauthorized: invalid JWT token", http.StatusUnauthorized)
		return
	}

	// Extract the organization claim
	org, err := c.extractOrgClaim(claims)
	if err != nil {
		log.Errorf("failed to extract org claim from JWT: %v", err)
		http.Error(rw, "Unauthorized: missing or invalid org claim", http.StatusUnauthorized)
		return
	}

	c.heartbeatHandler(rw, req, &org)
}

// heartbeatHandler is the common implementation for both API key and JWT protected endpoints
func (c *Client) heartbeatHandler(rw http.ResponseWriter, req *http.Request, org *string) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  static.WebsocketBufferSize,
		WriteBufferSize: static.WebsocketBufferSize,
	}
	ws, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		log.Errorf("failed to establish a connection: %v", err)
		metrics.RequestsTotal.WithLabelValues("heartbeat", "establish connection",
			"error upgrading the HTTP server connection to the WebSocket protocol").Inc()
		return
	}
	metrics.RequestsTotal.WithLabelValues("heartbeat", "establish connection", "OK").Inc()
	go c.handleHeartbeats(ws, org)
}

// handleHeartbeats handles incoming messages from the connection.
// If org is provided, it validates that registration hostnames belong to the specified organization.
func (c *Client) handleHeartbeats(ws conn, org *string) error {
	defer ws.Close()
	setReadDeadline(ws)

	var hostname string
	var experiment string
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			closeConnection(experiment, err)
			return err
		}
		if message != nil {
			setReadDeadline(ws)

			var hbm v2.HeartbeatMessage
			if err := json.Unmarshal(message, &hbm); err != nil {
				log.Errorf("failed to unmarshal heartbeat message, err: %v", err)
				continue
			}

			switch {
			case hbm.Registration != nil:
				// Validate organization if JWT authentication is used
				if org != nil {
					if err := c.validateOrganization(*org, hbm.Registration.Hostname); err != nil {
						log.Errorf("organization validation failed: %v", err)
						closeConnection(experiment, fmt.Errorf("organization validation failed: %w", err))
						return fmt.Errorf("organization validation failed: %w", err)
					}
				}

				if err := c.RegisterInstance(*hbm.Registration); err != nil {
					closeConnection(experiment, err)
					return err
				}

				if hostname == "" {
					hostname = hbm.Registration.Hostname
					experiment = hbm.Registration.Experiment
					metrics.CurrentHeartbeatConnections.WithLabelValues(experiment).Inc()
				}

				// Update Prometheus signals every time a Registration message is received.
				c.UpdatePrometheusForMachine(context.Background(), hbm.Registration.Hostname)
			case hbm.Health != nil:
				// Validate organization for health updates if JWT authentication is used
				if org != nil && hostname != "" {
					if err := c.validateOrganization(*org, hostname); err != nil {
						log.Errorf("organization validation failed for health update: %v", err)
						closeConnection(experiment, fmt.Errorf("organization validation failed: %w", err))
						return fmt.Errorf("organization validation failed: %w", err)
					}
				}

				if err := c.UpdateHealth(hostname, *hbm.Health); err != nil {
					closeConnection(experiment, err)
					return err
				}
			}
		}
	}
}

// setReadDeadline sets/resets the read deadline for the connection.
func setReadDeadline(ws conn) {
	deadline := time.Now().Add(readDeadline)
	ws.SetReadDeadline(deadline)
}

func closeConnection(experiment string, err error) {
	if experiment != "" {
		metrics.CurrentHeartbeatConnections.WithLabelValues(experiment).Dec()
	}
	log.Errorf("closing connection, err: %v", err)
}

// extractJWTClaims extracts JWT claims from the X-Endpoint-API-UserInfo header.
// This header is set by Cloud Endpoints (ESPv1) after successful JWT validation.
func (c *Client) extractJWTClaims(req *http.Request) (map[string]interface{}, error) {
	// Get the X-Endpoint-API-UserInfo header set by Cloud Endpoints
	userInfoHeader := req.Header.Get("X-Endpoint-API-UserInfo")
	if userInfoHeader == "" {
		return nil, fmt.Errorf("request must be processed through Cloud Endpoints: X-Endpoint-API-UserInfo header not found")
	}

	// Decode the base64url-encoded header content
	decoded, err := base64.RawURLEncoding.DecodeString(userInfoHeader)
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

	claims, ok := claimsInterface.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("claims field is not a valid JSON object")
	}

	return claims, nil
}

// extractOrgClaim extracts the "org" claim from JWT claims
func (c *Client) extractOrgClaim(claims map[string]interface{}) (string, error) {
	orgClaim, ok := claims["org"]
	if !ok {
		return "", fmt.Errorf("org claim not found in JWT")
	}

	org, ok := orgClaim.(string)
	if !ok {
		return "", fmt.Errorf("org claim is not a string")
	}

	if org == "" {
		return "", fmt.Errorf("org claim is empty")
	}

	return org, nil
}

// validateOrganization validates that the hostname belongs to the specified organization
func (c *Client) validateOrganization(org, hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname is empty")
	}

	// Parse the hostname using M-Lab's host package
	parsed, err := host.Parse(hostname)
	if err != nil {
		return fmt.Errorf("failed to parse hostname %s: %w", hostname, err)
	}

	// Compare the organization from the hostname with the JWT org claim
	if parsed.Org != org {
		return fmt.Errorf("organization mismatch: JWT org=%s, hostname org=%s (hostname: %s)",
			org, parsed.Org, hostname)
	}

	return nil
}
