package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/gomodule/redigo/redis"
	"github.com/justinas/alice"
	promet "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/access/token"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/limits"
	"github.com/m-lab/locate/memorystore"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/prometheus"
	"github.com/m-lab/locate/secrets"
	"github.com/m-lab/locate/static"
)

var (
	listenPort         string
	project            string
	platform           string
	locatorAE          bool
	locatorMM          bool
	legacyServer       string
	signerSecretName   string
	maxmind            = flagx.URL{}
	verifySecretName   string
	redisAddr          string
	promUserSecretName string
	promPassSecretName string
	promURL            string
	keySource          = flagx.Enum{
		Options: []string{"secretmanager", "local"},
		Value:   "secretmanager",
	}
	agentLimits    flagx.KeyValueEscaped
	durationLimits flagx.DurationArray
)

func init() {
	// PORT and GOOGLE_CLOUD_PROJECT are part of the default App Engine environment.
	flag.StringVar(&listenPort, "port", "8080", "AppEngine port environment variable")
	flag.StringVar(&project, "google-cloud-project", "", "AppEngine project environment variable")
	flag.StringVar(&platform, "platform-project", "", "GCP project for platform machine names")
	flag.StringVar(&signerSecretName, "signer-secret-name", "locate-service-signer-key", "Name of secret for locate signer key in Secret Manager")
	flag.StringVar(&verifySecretName, "verify-secret-name", "locate-monitoring-service-verify-key", "Name of secret for monitoring verifier key in Secret Manager")
	flag.StringVar(&redisAddr, "redis-address", "", "Primary endpoint for Redis instance")
	flag.StringVar(&promUserSecretName, "prometheus-username-secret-name", "prometheus-support-build-prom-auth-user",
		"Name of secret for Prometheus username")
	flag.StringVar(&promPassSecretName, "prometheus-password-secret-name", "prometheus-support-build-prom-auth-pass",
		"Name of secret for Prometheus password")
	flag.StringVar(&promURL, "prometheus-url", "", "Base URL to query prometheus")
	flag.BoolVar(&locatorAE, "locator-appengine", true, "Use the AppEngine clientgeo locator")
	flag.BoolVar(&locatorMM, "locator-maxmind", false, "Use the MaxMind clientgeo locator")
	flag.Var(&maxmind, "maxmind-url", "When -locator-maxmind is true, the tar URL of MaxMind IP database. May be: gs://bucket/file or file:./relativepath/file")
	flag.Var(&keySource, "key-source", "Where to load signer and verifier keys")
	flag.Var(&agentLimits, "agent-limits", "Cron schedule limits for user agents (agent=schedule).")
	flag.Var(&durationLimits, "duration-limits", "Time duration of client limits.")
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

type loader interface {
	LoadSigner(ctx context.Context, client secrets.SecretClient, name string) (*token.Signer, error)
	LoadVerifier(ctx context.Context, client secrets.SecretClient, name string) (*token.Verifier, error)
	LoadPrometheus(ctx context.Context, client secrets.SecretClient, user, pass string) (*prometheus.Credentials, error)
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	prom := prometheusx.MustServeMetrics()
	defer prom.Close()

	// Create the Secret Manager client
	client, err := secretmanager.NewClient(mainCtx)
	rtx.Must(err, "Failed to create Secret Manager client")
	var cfg loader

	switch keySource.Value {
	case "secretmanager":
		cfg = secrets.NewConfig(project)
	case "local":
		cfg = secrets.NewLocalConfig()
	}

	// SIGNER - load the signer key.
	signer, err := cfg.LoadSigner(mainCtx, client, signerSecretName)
	rtx.Must(err, "Failed to load signer key")

	locators := clientgeo.MultiLocator{clientgeo.NewUserLocator()}
	if locatorAE {
		aeLocator := clientgeo.NewAppEngineLocator()
		locators = append(locators, aeLocator)
	}
	if locatorMM {
		mm, err := content.FromURL(mainCtx, maxmind.URL)
		rtx.Must(err, "failed to load maxmindurl: %s", maxmind.URL)
		mmLocator := clientgeo.NewMaxmindLocator(mainCtx, mm)
		locators = append(locators, mmLocator)
	}

	pool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", redisAddr)
		},
	}
	memorystore := memorystore.NewClient[v2.HeartbeatMessage](&pool)
	tracker := heartbeat.NewHeartbeatStatusTracker(memorystore)
	defer tracker.StopImport()
	srvLocatorV2 := heartbeat.NewServerLocator(tracker)

	creds, err := cfg.LoadPrometheus(mainCtx, client, promUserSecretName, promPassSecretName)
	rtx.Must(err, "failed to load Prometheus credentials")
	promClient, err := prometheus.NewClient(creds, promURL)
	rtx.Must(err, "failed to create Prometheus client")

	if len(agentLimits.Get()) != len(durationLimits) {
		log.Fatal("Must provide the same number of agents limits as durations.")
	}

	lmts := make(map[string]*limits.Cron)
	i := 0
	for agent, schedule := range agentLimits.Get() {
		lmts[agent] = limits.NewCron(schedule, durationLimits[i])
		i++
	}
	c := handler.NewClient(project, signer, srvLocatorV2, locators, promClient, lmts)

	go func() {
		// Check and reload db at least once a day.
		reloadConfig := memoryless.Config{
			Min:      time.Hour,
			Max:      24 * time.Hour,
			Expected: 6 * time.Hour,
		}
		tick, err := memoryless.NewTicker(mainCtx, reloadConfig)
		rtx.Must(err, "Could not create ticker for reloading")
		for range tick.C {
			locators.Reload(mainCtx)
		}
	}()

	// MONITORING VERIFIER - for access tokens provided by monitoring.
	// The `verifier` returned by cfg.LoadVerifier() is a single object, but may
	// possibly itself contain multiple verification keys. The sequence for
	// getting here is something like: flag --verify-secret-name -> var
	// verifySecretName -> fetch all enabled secrets associated with name from
	// the Google Secret Manager -> pass a slice of JWT keys (secrets) to
	// token.NewVerifier(), which results in the `verifier` value assigned
	// below.
	verifier, err := cfg.LoadVerifier(mainCtx, client, verifySecretName)
	rtx.Must(err, "Failed to create verifier")
	exp := jwt.Expected{
		Issuer:   static.IssuerMonitoring,
		Audience: jwt.Audience{static.AudienceLocate},
	}
	tc, err := controller.NewTokenController(verifier, true, exp)
	rtx.Must(err, "Failed to create token controller")
	monitoringChain := alice.New(tc.Limit).Then(http.HandlerFunc(c.Monitoring))

	// Close the Secrent Manager client connection.
	client.Close()

	// TODO: add verifier for optional access tokens to support NextRequest.

	mux := http.NewServeMux()
	// PLATFORM APIs
	// Services report their health to the heartbeat service.
	mux.HandleFunc("/v2/platform/heartbeat", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/heartbeat"}),
		http.HandlerFunc(c.Heartbeat)))
	// Collect Prometheus health signals.
	mux.HandleFunc("/v2/platform/prometheus", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/prometheus"}),
		http.HandlerFunc(c.Prometheus)))
	// End to end monitoring requests access tokens for specific targets.
	mux.Handle("/v2/platform/monitoring/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/monitoring/"}),
		monitoringChain))

	// USER APIs
	// Clients request access tokens for specific services.
	mux.HandleFunc("/v2/nearest/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/nearest/"}),
		http.HandlerFunc(c.Nearest)))
	// REQUIRED: API keys parameters required for priority requests.
	mux.HandleFunc("/v2/priority/nearest/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/priority/nearest/"}),
		http.HandlerFunc(c.Nearest)))

	// Liveness and Readiness checks to support deployments.
	mux.HandleFunc("/v2/live", c.Live)
	mux.HandleFunc("/v2/ready", c.Ready)

	srv := &http.Server{
		Addr:    ":" + listenPort,
		Handler: mux,
	}
	log.Println("Listening for INSECURE access requests on " + listenPort)
	rtx.Must(httpx.ListenAndServeAsync(srv), "Could not start server")
	defer srv.Close()
	<-mainCtx.Done()
}
