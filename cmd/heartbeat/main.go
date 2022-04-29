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
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/static"
)

var (
	locate              string
	heartbeatPeriod     = static.HeartbeatPeriod
	mainCtx, mainCancel = context.WithCancel(context.Background())
)

func init() {
	flag.StringVar(&locate, "locate-url", "ws://localhost:8080/v2/platform/heartbeat/",
		"URL for locate service")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "failed to read args from env")

	conn := connection.NewConn()
	err := conn.Dial(locate, http.Header{})
	rtx.Must(err, "failed to establish a websocket connection with %s", locate)

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
			err := ws.WriteMessage(websocket.TextMessage, []byte("Health message!"))
			if err != nil {
				log.Printf("failed to write, err: %v", err)
			}
		}
	}
}
