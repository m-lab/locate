package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/cmd/heartbeat/registration"
	"github.com/m-lab/locate/static"
	log "github.com/sirupsen/logrus"
)

var (
	readDeadline = static.WebsocketReadDeadline
	instances    = make(map[string]*healthData)
)

type HeartbeatMessage struct {
	MsgType string           `json:"msgType"`
	Msg     *json.RawMessage `json:"msg"`
}

type healthData struct {
	instance registration.RegistrationMessage
	health   float64
}

type HealthMessage struct {
	Hostname string
	Score    float64
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

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Errorf("read error: %v", err)
			return
		}
		if message != nil {
			setReadDeadline(ws)

			var hbm HeartbeatMessage
			if err := json.Unmarshal(message, &hbm); err != nil {
				log.Errorf("failed to unmarshal message: %s, err: %v", string(message), err)
				continue
			}

			switch hbm.MsgType {
			case "registration":
				rm := new(registration.RegistrationMessage)
				if err := json.Unmarshal(*hbm.Msg, rm); err != nil {
					log.Errorf("failed to unmarshal registration message: %v, err: %v",
						hbm.Msg, err)
					return
				}
				instances[rm.Hostname] = &healthData{instance: *rm}
			case "health":
				hm := new(HealthMessage)
				if err := json.Unmarshal(*hbm.Msg, hm); err != nil {
					log.Errorf("failed to unmarshal health message: %v, err: %v",
						hbm.Msg, err)
					continue
				}
				instance, found := instances[hm.Hostname]
				if !found {
					log.Errorf("unavailable instance data for %s", hm.Hostname)
					continue
				}
				instance.health = hm.Score
			default:
				log.Errorf("unknown message type, type: %s", hbm.MsgType)
			}
		}
	}
}

// setReadDeadline sets/resets the read deadline for the connection.
func setReadDeadline(ws *websocket.Conn) {
	deadline := time.Now().Add(readDeadline)
	ws.SetReadDeadline(deadline)
}
