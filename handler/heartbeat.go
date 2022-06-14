package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/cmd/heartbeat/messaging"
	"github.com/m-lab/locate/static"
	log "github.com/sirupsen/logrus"
)

var (
	readDeadline = static.WebsocketReadDeadline
	instances    = make(map[string]*instanceData)
)

type instanceData struct {
	instance messaging.Registration
	health   float64
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
		return
	}
	go read(ws)
}

// read handles incoming messages from the connection.
func read(ws *websocket.Conn) {
	defer ws.Close()
	setReadDeadline(ws)
	registered := false

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Errorf("read error: %v", err)
			return
		}
		if message != nil {
			setReadDeadline(ws)

			if !registered {
				var rm messaging.Registration
				if err := json.Unmarshal(message, &rm); err != nil {
					log.Errorf("failed to unmarshal registration message, err: %v", err)
					return
				}
				instances[rm.Hostname] = &instanceData{instance: rm}
				registered = true
			} else {
				var hm messaging.Health
				if err := json.Unmarshal(message, &hm); err != nil {
					log.Errorf("failed to unmarshal health message, err: %v", err)
					continue
				}

				if instance, found := instances[hm.Hostname]; found {
					instance.health = hm.Score
				}
			}
		}
	}
}

// setReadDeadline sets/resets the read deadline for the connection.
func setReadDeadline(ws *websocket.Conn) {
	deadline := time.Now().Add(readDeadline)
	ws.SetReadDeadline(deadline)
}
