package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gomodule/redigo/redis"
	"github.com/justinas/alice"
	promet "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/access/token"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/auth/jwtverifier"
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
	limitsPath         string
	keySource          = flagx.Enum{
		Options: []string{"secretmanager", "local"},
		Value:   "secretmanager",
	}

	rateLimitRedisAddr    string
	rateLimitPrefix       string
	rateLimitIPUAInterval time.Duration
	rateLimitIPUAMax      int
	rateLimitIPInterval   time.Duration
	rateLimitIPMax        int

	earlyExitClients flagx.StringArray

	jwtAuthMode = flagx.Enum{
		Options: []string{"espv1", "direct", "insecure"},
		Value:   "espv1",
	}
	jwtJWKS = flagx.URL{}
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
	flag.StringVar(&limitsPath, "limits-path", "/go/src/github.com/m-lab/locate/limits/config.yaml", "Path to the limits config file")
	flag.DurationVar(&rateLimitIPUAInterval, "rate-limit-interval", time.Hour, "Time window for IP+UA rate limiting")
	flag.IntVar(&rateLimitIPUAMax, "rate-limit-max", 40, "Max number of events in the time window for IP+UA rate limiting")
	flag.DurationVar(&rateLimitIPInterval, "rate-limit-ip-interval", time.Hour,
		"Time window for IP-only rate limiting")
	flag.IntVar(&rateLimitIPMax, "rate-limit-ip-max", 120,
		"Max number of events in the time window for IP-only rate limiting")
	flag.StringVar(&rateLimitPrefix, "rate-limit-prefix", "locate:ratelimit", "Prefix for Redis keys for IP+UA rate limiting")
	flag.StringVar(&rateLimitRedisAddr, "rate-limit-redis-address", "", "Primary endpoint for Redis instance for rate limiting")
	flag.Var(&earlyExitClients, "early-exit-clients", "Client names that should receive early_exit parameter (can be specified multiple times)")
	flag.Var(&jwtAuthMode, "jwt-auth-mode", "JWT authentication mode: espv1 (Cloud Endpoints, production default), direct (JWKS validation for integration testing), insecure (dev/test only, requires ALLOW_INSECURE_JWT=true)")
	flag.Var(&jwtJWKS, "jwt-jwks-url", "JWKS URL for direct mode JWT verification (e.g., https://auth.example.com/.well-known/jwks.json)")
	// Enable logging with line numbers to trace error locations.
	log.SetFlags(log.LUTC | log.Llongfile)
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

type loader interface {
	LoadSigner(ctx context.Context, name string) (*token.Signer, error)
	LoadVerifier(ctx context.Context, name string) (*token.Verifier, error)
	LoadPrometheus(ctx context.Context, user, pass string) (*prometheus.Credentials, error)
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")
	defer mainCancel()

	prom := prometheusx.MustServeMetrics()
	defer prom.Close()

	// Create the Secret Manager client
	var cfg loader

	switch keySource.Value {
	case "secretmanager":
		client, err := secretmanager.NewClient(mainCtx)
		rtx.Must(err, "Failed to create Secret Manager client")
		cfg = secrets.NewConfig(project, client)
		defer client.Close()
	case "local":
		cfg = secrets.NewLocalConfig()
	}

	// SIGNER - load the signer key.
	signer, err := cfg.LoadSigner(mainCtx, signerSecretName)
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

	// Rate limiter Redis pool.
	rateLimitPool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", rateLimitRedisAddr)
		},
	}
	rateLimitConfig := limits.RateLimitConfig{
		IPConfig: limits.LimitConfig{
			Interval:  rateLimitIPInterval,
			MaxEvents: rateLimitIPMax,
		},
		IPUAConfig: limits.LimitConfig{
			Interval:  rateLimitIPUAInterval,
			MaxEvents: rateLimitIPUAMax,
		},
		KeyPrefix: rateLimitPrefix,
	}
	ipLimiter := limits.NewRateLimiter(&rateLimitPool, rateLimitConfig)

	creds, err := cfg.LoadPrometheus(mainCtx, promUserSecretName, promPassSecretName)
	rtx.Must(err, "failed to load Prometheus credentials")
	promClient, err := prometheus.NewClient(creds, promURL)
	rtx.Must(err, "failed to create Prometheus client")

	lmts, tierLmts, err := limits.ParseFullConfig(limitsPath)
	rtx.Must(err, "failed to parse limits config")

	// Create JWT verifier based on configured mode
	var jwtVerifier handler.Verifier
	switch jwtAuthMode.Value {
	case "espv1":
		jwtVerifier = jwtverifier.NewESPv1()
		log.Printf("Using JWT verification mode: espv1 (Cloud Endpoints)")
	case "direct":
		if jwtJWKS.URL == nil {
			rtx.Must(fmt.Errorf("--jwt-jwks-url is required for direct mode"), "JWT configuration error")
		}
		jwtVerifier, err = jwtverifier.NewDirect(jwtJWKS.URL)
		rtx.Must(err, "failed to create direct JWT verifier")
		log.Printf("Using JWT verification mode: direct (JWKS URL: %s)", jwtJWKS.URL)
	case "insecure":
		jwtVerifier, err = jwtverifier.NewInsecure()
		rtx.Must(err, "failed to create insecure JWT verifier")
		log.Printf("Using JWT verification mode: insecure (WARNING: No signature validation)")
	default:
		rtx.Must(fmt.Errorf("unknown JWT auth mode: %s", jwtAuthMode.Value), "JWT configuration error")
	}

	c := handler.NewClient(project, signer, srvLocatorV2, locators, promClient,
		lmts, tierLmts, ipLimiter, earlyExitClients, jwtVerifier)

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
	verifier, err := cfg.LoadVerifier(mainCtx, verifySecretName)
	rtx.Must(err, "Failed to create verifier")
	exp := jwt.Expected{
		Issuer:      static.IssuerMonitoring,
		AnyAudience: jwt.Audience{static.AudienceLocate},
	}

	// The /v2/platform/monitoring endpoint requires a token that is only
	// available to the monitoring tools. TokenController validates this token.
	// TODO: update m-lab/access to support prefix matching, then simplify to
	// just "/v2/platform/monitoring/".
	tc, err := controller.NewTokenController(verifier, true, exp, controller.Paths{
		"/v2/platform/monitoring/ndt/ndt7":    true,
		"/v2/platform/monitoring/wehe/replay": true,
	})
	rtx.Must(err, "Failed to create token controller")
	monitoringChain := alice.New(tc.Limit).Then(http.HandlerFunc(c.Monitoring))

	// TODO: add verifier for optional access tokens to support NextRequest.

	mux := http.NewServeMux()
	// PLATFORM APIs
	// Services report their health to the heartbeat service (API key protected).
	mux.HandleFunc("/v2/platform/heartbeat", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/heartbeat"}),
		http.HandlerFunc(c.Heartbeat)))
	// Services report their health to the heartbeat service (JWT protected with org validation).
	// JWT verification is handled by Cloud Endpoints.
	mux.HandleFunc("/v2/platform/heartbeat-jwt", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/heartbeat-jwt"}),
		http.HandlerFunc(c.HeartbeatJWT)))
	// Collect Prometheus health signals.
	mux.HandleFunc("/v2/platform/prometheus", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/prometheus"}),
		http.HandlerFunc(c.Prometheus)))
	// End to end monitoring requests access tokens for specific targets.
	mux.Handle("/v2/platform/monitoring/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/platform/monitoring/"}),
		monitoringChain))

	// USER APIs
	//
	// Clients request access tokens for specific services.
	mux.HandleFunc("/v2/nearest/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/nearest/"}),
		http.HandlerFunc(c.Nearest)))

	// REQUIRED: API keys parameters required for priority requests.
	mux.HandleFunc("/v2/priority/nearest/", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/v2/priority/nearest/"}),
		http.HandlerFunc(c.PriorityNearest)))

	// DEPRECATED USER APIs
	//
	// TODO(https://github.com/m-lab/locate/issues/185).
	mux.HandleFunc("/ndt", promhttp.InstrumentHandlerDuration(
		metrics.RequestHandlerDuration.MustCurryWith(promet.Labels{"path": "/ndt"}),
		http.HandlerFunc(c.MLabNSCompat)))

	// Liveness and Readiness checks to support deployments.
	mux.HandleFunc("/v2/live", c.Live)
	mux.HandleFunc("/v2/ready", c.Ready)

	// Return list of all heartbeat registrations
	mux.HandleFunc("/v2/siteinfo/registrations", c.Registrations)

	srv := &http.Server{
		Addr:    ":" + listenPort,
		Handler: mux,
	}
	log.Println("Listening for INSECURE access requests on " + listenPort)
	rtx.Must(httpx.ListenAndServeAsync(srv), "Could not start server")
	defer srv.Close()
	<-mainCtx.Done()
}
