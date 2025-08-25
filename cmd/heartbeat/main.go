package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	md "cloud.google.com/go/compute/metadata"
	"github.com/gorilla/websocket"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/cmd/heartbeat/health"
	"github.com/m-lab/locate/cmd/heartbeat/metadata"
	"github.com/m-lab/locate/cmd/heartbeat/registration"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
)

var (
	heartbeatURL        string
	hostname            flagx.StringFile
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
	lbPath              = "/metadata/loadbalanced"

	// JWT authentication parameters
	apiKey           string
	tokenExchangeURL string
)

// Checker generates a health score for the heartbeat instance (0, 1).
type Checker interface {
	GetHealth(ctx context.Context) float64 // Health score.
}

// TokenResponse represents the response from the token exchange service
type TokenResponse struct {
	Token string `json:"token"`
}

// getJWTTokenFunc is a variable that holds the JWT token function, allowing for test overrides
var getJWTTokenFunc = getJWTToken

// getJWTToken exchanges an API key for a JWT token
func getJWTToken(apiKey, tokenExchangeURL string) (string, error) {
	// Prepare the request payload
	payload := map[string]string{
		"api_key": apiKey,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	// Make the HTTP request to the token exchange service
	resp, err := http.Post(tokenExchangeURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to request JWT token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}

	// Parse the response
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Token == "" {
		return "", fmt.Errorf("empty token received from exchange service")
	}

	return tokenResp.Token, nil
}

func init() {
	flag.StringVar(&heartbeatURL, "heartbeat-url", "ws://localhost:8080/v2/platform/heartbeat",
		"URL for locate service")
	flag.Var(&hostname, "hostname", "The service hostname (may be read from @/path/file)")
	flag.StringVar(&experiment, "experiment", "", "Experiment name")
	flag.StringVar(&pod, "pod", "", "Kubernetes pod name")
	flag.StringVar(&node, "node", "", "Kubernetes node name")
	flag.StringVar(&namespace, "namespace", "", "Kubernetes namespace")
	flag.Var(&kubernetesURL, "kubernetes-url", "URL for Kubernetes API")
	flag.Var(&registrationURL, "registration-url", "URL for site registration")
	flag.Var(&services, "services", "Maps experiment target names to their set of services")
	flag.StringVar(&apiKey, "api-key", "", "API key for JWT token exchange (required)")
	flag.StringVar(&tokenExchangeURL, "token-exchange-url", "https://auth.mlab-sandbox.measurementlab.net/v0/token/autojoin",
		"URL for token exchange service")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "failed to read args from env")

	// Validate JWT authentication parameters
	if apiKey == "" {
		log.Fatal("API key is required for JWT authentication (-api-key flag)")
	}
	if tokenExchangeURL == "" {
		log.Fatal("Token exchange URL is required (-token-exchange-url flag)")
	}

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
	ldr, err := registration.NewLoader(mainCtx, registrationURL.URL, hostname.Value, experiment, svcs, ldrConfig)
	rtx.Must(err, "could not initialize registration loader")
	r, err := ldr.GetRegistration(mainCtx)
	rtx.Must(err, "could not load registration data")
	hbm := v2.HeartbeatMessage{Registration: r}

	// Get JWT token for authentication
	log.Printf("Exchanging API key for JWT token...")
	jwtToken, err := getJWTTokenFunc(apiKey, tokenExchangeURL)
	rtx.Must(err, "failed to get JWT token")
	log.Printf("Successfully obtained JWT token")

	// Prepare headers with JWT authentication
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+jwtToken)

	// Establish a connection with JWT authentication.
	conn := connection.NewConn()

	// Set up JWT token refresh for automatic token renewal
	conn.SetTokenRefresher(func() (string, error) {
		log.Printf("Refreshing JWT token...")
		return getJWTTokenFunc(apiKey, tokenExchangeURL)
	})

	err = conn.Dial(heartbeatURL, headers, hbm)
	rtx.Must(err, "failed to establish a websocket connection with %s", heartbeatURL)

	probe := health.NewPortProbe(svcs)
	ec := health.NewEndpointClient(static.HealthEndpointTimeout)
	var hc Checker

	// TODO(kinkade): cause a fatal error if lberr is not nil. Not fatally
	// exiting on lberr is just a workaround to get this rolled out while we
	// wait for every physical machine on the platform to actually have that
	// file, which won't be the case until the rolling reboot in production
	// completes in 4 or 5 days, as of this comment 2024-08-06.
	lbbytes, lberr := os.ReadFile(lbPath)

	// If the "loadbalanced" file exists, then make sure that the content of the
	// file is "true". If the file doesn't exist, then, for now, just consider
	// the machine as not loadbalanced.
	if lberr == nil && string(lbbytes) == "true" {
		gcpmd, err := metadata.NewGCPMetadata(md.NewClient(http.DefaultClient), hostname.Value)
		rtx.Must(err, "failed to get VM metadata")
		gceClient, err := compute.NewRegionBackendServicesRESTClient(mainCtx)
		rtx.Must(err, "failed to create GCE client")
		hc = health.NewGCPChecker(gceClient, gcpmd)
	} else if kubernetesURL.URL == nil {
		hc = health.NewChecker(probe, ec)
	} else {
		k8s := health.MustNewKubernetesClient(kubernetesURL.URL, pod, node, namespace, kubernetesAuth)
		hc = health.NewCheckerK8S(probe, k8s, ec)
	}

	write(conn, hc, ldr)
}

// write starts a write loop to send health messages every
// HeartbeatPeriod.
func write(ws *connection.Conn, hc Checker, ldr *registration.Loader) {
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

func getHealth(hc Checker) float64 {
	ctx, cancel := context.WithTimeout(mainCtx, heartbeatPeriod)
	defer cancel()
	return hc.GetHealth(ctx)
}

func sendMessage(ws *connection.Conn, hbm v2.HeartbeatMessage, msgType string) {
	// If a new registration message was found, update the websocket's dial message.
	// The message is sent whenever the connection is restarted (i.e., once per hour in App Engine).
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
