package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
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

// Heartbeat implements /v2/heartbeat requests.
// It starts a new persistent connection and a new goroutine
// to read incoming messages.
func (c *Client) Heartbeat(rw http.ResponseWriter, req *http.Request) {
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
	go c.handleHeartbeats(ws)
}

// handleHeartbeats handles incoming messages from the connection.
func (c *Client) handleHeartbeats(ws conn) error {
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
				c.promRegistration(context.Background(), hbm.Registration.Hostname)
			case hbm.Health != nil:
				if err := c.UpdateHealth(hostname, *hbm.Health); err != nil {
					closeConnection(experiment, err)
					return err
				}
			}
		}
	}
}

func (c *Client) promRegistration(ctx context.Context, host string) error {
	if err := c.UpdatePrometheusForMachine(ctx, host); err != nil {
		log.Printf("Failed to update Prometheus after Registration: %v", err)
		return err
	}
	return nil
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
