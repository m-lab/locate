package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/host"
	"github.com/m-lab/go/prometheusx"
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

	// Start metrics server.
	prom := prometheusx.MustServeMetrics()
	defer prom.Close()

	// Parse the node's name into its constituent parts. This ensures that the
	// value of the -hostname flag is actually valid. Additionally, virtual
	// nodes which are part of a managed instance group may have a random
	// suffix, which Locate cannot use, so we explicitly only include
	// the parts of the node name that Locate actually cares about. The
	// resultant variable mlabHostname should match a machine name in siteinfo's
	// registration.json:
	//
	// https://siteinfo.mlab-oti.measurementlab.net/v2/sites/registration.json
	h, err := host.Parse(hostname)
	rtx.Must(err, "Failed to parse -hostname flag value")
	mlabHostname := h.String()

	// Load registration data.
	r, err := LoadRegistration(mainCtx, mlabHostname, registrationURL.URL)
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
	hc := &health.Checker{}
	if kubernetesURL.URL == nil {
		hc = health.NewChecker(probe)
	} else {
		k8s := health.MustNewKubernetesClient(kubernetesURL.URL, pod, node, namespace, kubernetesAuth)
		hc = health.NewCheckerK8S(probe, k8s)
	}

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
			score := getHealth(hc)
			healthMsg := v2.Health{Score: score}
			hbm := v2.HeartbeatMessage{Health: &healthMsg}

			err := ws.WriteMessage(websocket.TextMessage, hbm)
			if err != nil {
				log.Printf("failed to write health message, err: %v", err)
			}
		}
	}
}

func getHealth(hc *health.Checker) float64 {
	ctx, cancel := context.WithTimeout(mainCtx, heartbeatPeriod)
	defer cancel()
	return hc.GetHealth(ctx)
}
