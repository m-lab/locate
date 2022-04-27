package main

import (
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

var locate string

func init() {
	flag.StringVar(&locate, "locate-url", "ws://localhost:8080/v2/platform/heartbeat/",
		"URL for locate service")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "Failed to read args from env")

	conn := connection.NewConn()
	conn.Dial(locate, http.Header{})
	write(conn)
}

// write starts a write loop to send health messages every
// HeartbeatPeriod.
func write(ws *connection.Conn) {
	defer ws.Close()
	ticker := time.NewTicker(static.HeartbeatPeriod)
	defer ticker.Stop()

	for {
		<-ticker.C
		err := ws.WriteMessage(websocket.TextMessage, []byte("Health message!"))
		if err != nil {
			log.Printf("failed to write, err: %v", err)
		}
	}
}
