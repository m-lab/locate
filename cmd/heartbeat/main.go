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
	"github.com/m-lab/locate/cmd/heartbeat/messaging"
	"github.com/m-lab/locate/connection"
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
	r, err := messaging.LoadRegistration(mainCtx, hostname, registrationURL.URL)
	rtx.Must(err, "could not load registration data")
	b, err := json.Marshal(r)
	rtx.Must(err, "failed to marshal registration message, msg: %v", r)

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
			healthMsg := messaging.Health{
				Hostname: hostname,
				Score:    1.0,
			}
			b, err := json.Marshal(healthMsg)
			if err == nil {
				err = ws.WriteMessage(websocket.TextMessage, b)
			}
			if err != nil {
				log.Printf("failed to write health message, err: %v", err)
			}
		}
	}
}
