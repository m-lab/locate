package handler

import (
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
		metrics.RequestsTotal.WithLabelValues("heartbeat", err.Error()).Inc()
		return
	}

	// When the /heartbeat request is first received, the 'experiment' is not known.
	metrics.CurrentHeartbeatConnections.WithLabelValues("").Inc()
	metrics.RequestsTotal.WithLabelValues("heartbeat", "OK").Inc()
	go c.handleHeartbeats(ws)
}

// handleHeartbeats handles incoming messages from the connection.
func (c *Client) handleHeartbeats(ws *websocket.Conn) {
	defer ws.Close()
	setReadDeadline(ws)

	var hostname string
	var experiment string
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Errorf("read error: %v", err)
			metrics.CurrentHeartbeatConnections.WithLabelValues(experiment).Dec()
			return
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
				hostname, experiment = c.handleRegistration(hbm.Registration)
			case hbm.Health != nil:
				c.UpdateHealth(hostname, *hbm.Health)
			}
		}
	}
}

// handleRegistration registers the instance and updates the corresponding metrics
// on receipt of a v2.Registration message.
func (c *Client) handleRegistration(rm *v2.Registration) (string, string) {
	c.RegisterInstance(*rm)

	// Once the registration message is received, decrement the counter
	// for the unknown experiment and increase it with the experiment in
	// the registration.
	experiment := rm.Experiment
	metrics.CurrentHeartbeatConnections.WithLabelValues("").Dec()
	metrics.CurrentHeartbeatConnections.WithLabelValues(experiment).Inc()

	return rm.Hostname, experiment
}

// setReadDeadline sets/resets the read deadline for the connection.
func setReadDeadline(ws *websocket.Conn) {
	deadline := time.Now().Add(readDeadline)
	ws.SetReadDeadline(deadline)
}
