package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/static"
)

var (
	heartbeatURL        string
	hostname            string
	experiment          string
	registrationURL     = flagx.URL{}
	services            = flagx.KeyValueArray{}
	heartbeatPeriod     = static.HeartbeatPeriod
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func init() {
	flag.StringVar(&heartbeatURL, "heartbeat-url", "ws://localhost:8080/v2/platform/heartbeat",
		"URL for locate service")
	flag.StringVar(&hostname, "hostname", "", "The service hostname")
	flag.StringVar(&experiment, "experiment", "", "Experiment name")
	flag.Var(&registrationURL, "registration-url", "URL for site registration")
	flag.Var(&services, "services", "Maps experiment target names to their set of services")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "failed to read args from env")

	// Load registration data.
	r, err := LoadRegistration(mainCtx, hostname, registrationURL.URL)
	rtx.Must(err, "could not load registration data")
	// Populate flag values.
	r.Experiment = experiment
	r.Services = services.Get()

	// Establish a connection.
	c := connection.NewConn()
	err = c.Dial(heartbeatURL, http.Header{}, r)
	rtx.Must(err, "failed to establish a websocket connection with %s", heartbeatURL)

	write(c)
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
			healthMsg := v2.Health{
				Hostname: hostname,
				Score:    1.0,
			}
			err := ws.WriteMessage(websocket.TextMessage, healthMsg)
			if err != nil {
				log.Printf("failed to write health message, err: %v", err)
			}
		}
	}
}
