package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/cmd/heartbeat/registration"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/static"
)

var (
	heartbeatURL        string
	hostname            string
	registrationURL     = flagx.URL{}
	heartbeatPeriod     = static.HeartbeatPeriod
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func init() {
	flag.StringVar(&heartbeatURL, "heartbeat-url", "ws://localhost:8080/v2/platform/heartbeat",
		"URL for locate service")
	flag.StringVar(&hostname, "hostname", "", "The service hostname")
	flag.Var(&registrationURL, "registration-url", "URL for site registration")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "failed to read args from env")

	// Load registration data.
	r, err := registration.Load(mainCtx, hostname, registrationURL.URL)
	rtx.Must(err, "could not load registration data")
	jsonMsg, err := json.Marshal(r)
	rtx.Must(err, "failed to marshal registration message, msg: %v", r)
	rawMsg := json.RawMessage(jsonMsg)
	b, err := constructHeartbeatMsg("registration", rawMsg)
	rtx.Must(err, "failed to construct registraion message")

	// Establish a connection.
	conn := connection.NewConn(b)
	err = conn.Dial(heartbeatURL, http.Header{})
	rtx.Must(err, "failed to establish a websocket connection with %s", heartbeatURL)

	write(conn)
}

// write starts a write loop to send health messages every
// HeartbeatPeriod.
func write(ws *connection.Conn) {
	defer ws.Close()
	ticker := *time.NewTicker(heartbeatPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-mainCtx.Done():
			return
		case <-ticker.C:
			healthMsg := json.RawMessage(`{
				"Hostname": "` + hostname + `",
				"Score": 1
			}`)

			b, err := constructHeartbeatMsg("health", healthMsg)
			if err == nil {
				err = ws.WriteMessage(websocket.TextMessage, b)
				if err != nil {
					log.Printf("failed to write health message, err: %v", err)
				}
			}
		}
	}
}

// constructHeartbeatMsg uses a message type (i.e., "health" or "registration")
// and a raw message to construct the messages sent by the heartbeat service.
func constructHeartbeatMsg(msgType string, msg json.RawMessage) ([]byte, error) {
	hbm := handler.HeartbeatMessage{
		MsgType: msgType,
		Msg:     &msg,
	}
	b, err := json.Marshal(hbm)
	if err != nil {
		log.Printf("failed to marshal heartbeat message, msgType: %s, err: %v",
			msgType, err)
		return nil, err
	}
	return b, nil
}
