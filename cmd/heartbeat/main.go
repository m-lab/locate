package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/cmd/heartbeat/health"
	"github.com/m-lab/locate/cmd/heartbeat/registration"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/metrics"
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

	// Load registration data.
	ldrConfig := memoryless.Config{
		Min:      static.RegistrationLoadMin,
		Expected: static.RegistrationLoadExpected,
		Max:      static.RegistrationLoadMax,
	}
	svcs := services.Get()
	ldr, err := registration.NewLoader(mainCtx, registrationURL.URL, hostname, experiment, svcs, ldrConfig)
	rtx.Must(err, "could not initialize registration loader")
	r, err := ldr.GetRegistration(mainCtx)
	rtx.Must(err, "could not load registration data")
	hbm := v2.HeartbeatMessage{Registration: r}

	// Establish a connection.
	conn := connection.NewConn()
	err = conn.Dial(heartbeatURL, http.Header{}, hbm)
	rtx.Must(err, "failed to establish a websocket connection with %s", heartbeatURL)

	probe := health.NewPortProbe(svcs)
	ec := health.NewEndpointClient(static.HealthEndpointTimeout)
	hc := &health.Checker{}
	if kubernetesURL.URL == nil {
		hc = health.NewChecker(probe, ec)
	} else {
		k8s := health.MustNewKubernetesClient(kubernetesURL.URL, pod, node, namespace, kubernetesAuth)
		hc = health.NewCheckerK8S(probe, k8s, ec)
	}

	write(conn, hc, ldr)
}

// write starts a write loop to send health messages every
// HeartbeatPeriod.
func write(ws *connection.Conn, hc *health.Checker, ldr *registration.Loader) {
	defer ws.Close()
	hbTicker := *time.NewTicker(heartbeatPeriod)
	defer hbTicker.Stop()

	// Register the channel to receive SIGTERM events.
	sigterm := make(chan os.Signal, 1)
	defer close(sigterm)
	signal.Notify(sigterm, syscall.SIGTERM)

	defer ldr.Ticker.Stop()

	for {
		select {
		case <-mainCtx.Done():
			log.Println("context cancelled")
			sendExitMessage(ws)
			return
		case <-sigterm:
			log.Println("received SIGTERM")
			sendExitMessage(ws)
			mainCancel()
			return
		case <-ldr.Ticker.C:
			reg, err := ldr.GetRegistration(mainCtx)
			if err != nil {
				log.Printf("could not load registration data, err: %v", err)
			}
			if reg != nil {
				sendMessage(ws, v2.HeartbeatMessage{Registration: reg}, "registration")
				log.Printf("updated registration to %v", reg)
			}
		case <-hbTicker.C:
			t := time.Now()
			score := getHealth(hc)
			healthMsg := v2.Health{Score: score}
			hbm := v2.HeartbeatMessage{Health: &healthMsg}
			sendMessage(ws, hbm, "health")

			// Record duration metric.
			fmtScore := fmt.Sprintf("%.1f", score)
			metrics.HealthTransmissionDuration.WithLabelValues(fmtScore).Observe(time.Since(t).Seconds())
		}
	}
}

func getHealth(hc *health.Checker) float64 {
	ctx, cancel := context.WithTimeout(mainCtx, heartbeatPeriod)
	defer cancel()
	return hc.GetHealth(ctx)
}

func sendMessage(ws *connection.Conn, hbm v2.HeartbeatMessage, msgType string) {
	// If a new registration message was found, update the websocket's dial message.
	// The message sent whenever the connection is restarted (i.e., once per hour in App Engine).
	if msgType == "registration" {
		ws.DialMessage = hbm
	}

	err := ws.WriteMessage(websocket.TextMessage, hbm)
	if err != nil {
		log.Printf("failed to write %s message, err: %v", msgType, err)
	}
}

func sendExitMessage(ws *connection.Conn) {
	// Notify the receiver that the health score should now be 0.
	hbm := v2.HeartbeatMessage{
		Health: &v2.Health{
			Score: 0,
		},
	}
	sendMessage(ws, hbm, "final health")
}
