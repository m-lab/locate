package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	v2 "github.com/m-lab/locate/api/v2"
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
		return
	}
	go c.handleHeartbeats(ws)
}

// handleHeartbeats handles incoming messages from the connection.
func (c *Client) handleHeartbeats(ws *websocket.Conn) {
	defer ws.Close()
	setReadDeadline(ws)

	var hostname string
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Errorf("read error: %v", err)
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
				c.registerInstance(*hbm.Registration)
				hostname = hbm.Registration.Hostname
			case hbm.Health != nil:
				c.updateScore(hostname, *hbm.Health)
			}
		}
	}
}

func (c *Client) registerInstance(rm v2.Registration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances[rm.Hostname] = &v2.HeartbeatMessage{Registration: &rm}
}

func (c *Client) updateScore(hostname string, hm v2.Health) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if instance, found := c.instances[hostname]; found {
		instance.Health = &hm
	}
}

// setReadDeadline sets/resets the read deadline for the connection.
func setReadDeadline(ws *websocket.Conn) {
	deadline := time.Now().Add(readDeadline)
	ws.SetReadDeadline(deadline)
}
