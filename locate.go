package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/justinas/alice"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/content"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/memoryless"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/secrets"
	"github.com/m-lab/locate/static"
)

var (
	listenPort       string
	project          string
	platform         string
	locatorAE        bool
	locatorMM        bool
	legacyServer     string
	signerSecretName string
	maxmind          = flagx.URL{}
	verifySecretName string
)

func init() {
	// PORT and GOOGLE_CLOUD_PROJECT are part of the default App Engine environment.
	flag.StringVar(&listenPort, "port", "8080", "AppEngine port environment variable")
	flag.StringVar(&project, "google-cloud-project", "", "AppEngine project environment variable")
	flag.StringVar(&platform, "platform-project", "", "GCP project for platform machine names")
	flag.StringVar(&legacyServer, "legacy-server", proxy.DefaultLegacyServer, "Base URL to mlab-ns server")
	flag.StringVar(&signerSecretName, "signer-secret-name", "locate-service-signer-key", "Name of secret for locate signer key in Secret Manager")
	flag.StringVar(&verifySecretName, "verify-secret-name", "locate-monitoring-service-verify-key", "Name of secret for monitoring verifier key in Secret Manager")
	flag.BoolVar(&locatorAE, "locator-appengine", true, "Use the AppEngine clientgeo locator")
	flag.BoolVar(&locatorMM, "locator-maxmind", false, "Use the MaxMind clientgeo locator")
	flag.Var(&maxmind, "maxmind-url", "When -locator-maxmind is true, the tar URL of MaxMind IP database. May be: gs://bucket/file or file:./relativepath/file")
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	// Create the Secret Manager client
	client, err := secretmanager.NewClient(mainCtx)
	rtx.Must(err, "Failed to create Secret Manager client")
	cfg := secrets.NewConfig(project)

	// SIGNER - load the signer key.
	cfg.Name = signerSecretName
	signer, err := cfg.LoadSigner(mainCtx, client)
	rtx.Must(err, "Failed to load signer key")

	srvLocator := proxy.MustNewLegacyLocator(legacyServer, platform)

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
	c := handler.NewClient(project, signer, srvLocator, locators)

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
	cfg.Name = verifySecretName
	verifier, err := cfg.LoadVerifier(mainCtx, client)
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

	// TODO: add verifier for heartbeat access tokens.
	// TODO: add verifier for optional access tokens to support NextRequest.

	mux := http.NewServeMux()
	// PLATFORM APIs
	// Services report their health to the heartbeat service.
	mux.HandleFunc("/v2/platform/heartbeat/", http.HandlerFunc(c.Heartbeat))
	// End to end monitoring requests access tokens for specific targets.
	mux.Handle("/v2/platform/monitoring/", monitoringChain)

	// USER APIs
	// Clients request access tokens for specific services.
	mux.HandleFunc("/v2/nearest/", http.HandlerFunc(c.TranslatedQuery))
	// REQUIRED: API keys parameters required for priority requests.
	mux.HandleFunc("/v2/priority/nearest/", http.HandlerFunc(c.TranslatedQuery))

	// DEPRECATED APIs: TODO: retire after migrating clients.
	mux.Handle("/v2/monitoring/", monitoringChain)
	mux.HandleFunc("/v2beta1/query/", http.HandlerFunc(c.TranslatedQuery))

	srv := &http.Server{
		Addr:    ":" + listenPort,
		Handler: mux,
	}
	log.Println("Listening for INSECURE access requests on " + listenPort)
	rtx.Must(httpx.ListenAndServeAsync(srv), "Could not start server")
	defer srv.Close()
	<-mainCtx.Done()
}
