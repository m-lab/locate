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
	"github.com/m-lab/locate/cmd/heartbeat/health"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/static"
)

var (
	heartbeatURL        string
	hostname            string
	experiment          string
	pod                 string
	node                string
	namespace           string
	kubernetesAuth      = "/var/run/secrets/kubernetes.io/serviceaccount/"
	kubernetesURL       = flagx.URL{}
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
	flag.StringVar(&pod, "pod", "", "Kubernetes pod name")
	flag.StringVar(&node, "node", "", "Kubernetes node name")
	flag.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flag.Var(&kubernetesURL, "kubernetes-url", "URL for Kubernetes API")
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
	s := services.Get()
	r.Services = s
	r.Experiment = experiment
	hbm := v2.HeartbeatMessage{Registration: r}

	// Establish a connection.
	conn := connection.NewConn()
	err = conn.Dial(heartbeatURL, http.Header{}, hbm)
	rtx.Must(err, "failed to establish a websocket connection with %s", heartbeatURL)

	probe := health.NewPortProbe(s)
	k8s := health.MustNewKubernetesClient(mainCtx, kubernetesURL.URL, pod, node, namespace, kubernetesAuth)
	hc := health.NewChecker(probe, k8s)

	write(conn, hc)
}

// write starts a write loop to send health messages every
// HeartbeatPeriod.
func write(ws *connection.Conn, hc *health.Checker) {
	defer ws.Close()
	ticker := *time.NewTicker(heartbeatPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-mainCtx.Done():
			return
		case <-ticker.C:
			score := hc.GetHealth()
			healthMsg := v2.Health{Score: score}
			hbm := v2.HeartbeatMessage{Health: &healthMsg}

			err := ws.WriteMessage(websocket.TextMessage, hbm)
			if err != nil {
				log.Printf("failed to write health message, err: %v", err)
			}
		}
	}
}
