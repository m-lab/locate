package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/justinas/alice"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/decrypt"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/static"
)

var (
	listenPort          string
	project             string
	locateSignerKey     string
	monitoringVerifyKey string
)

func init() {
	// PORT and GOOGLE_CLOUD_PROJECT are part of the default App Engine environment.
	flag.StringVar(&listenPort, "port", "8080", "AppEngine port environment variable")
	flag.StringVar(&project, "google-cloud-project", "", "AppEngine project environment variable")
	flag.StringVar(&locateSignerKey, "locate-signer-key", "", "Private key of the locate+service key pair")
	flag.StringVar(&monitoringVerifyKey, "monitoring-verify-key", "", "Public key of the monitoring+locate key pair")
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	// SIGNER - load the signer key.
	client, err := kms.NewKeyManagementClient(mainCtx)
	rtx.Must(err, "Failed to create KMS client")
	// NOTE: these must be the same parameters used by management/create_encrypted_signer_key.sh.
	cfg := decrypt.NewConfig(project, "global", "locate-signer", "jwk")
	// Load encrypted signer key from environment, using variable name derived from project.
	signer, err := cfg.LoadSigner(mainCtx, client, locateSignerKey)
	rtx.Must(err, "Failed to load signer key")
	locator := proxy.MustNewLegacyLocator(proxy.DefaultLegacyServer)
	c := handler.NewClient(project, signer, locator)

	// MONITORING VERIFIER - for access tokens provided by monitoring.
	verifier, err := cfg.LoadVerifier(mainCtx, client, monitoringVerifyKey)
	rtx.Must(err, "Failed to create verifier")
	exp := jwt.Expected{
		Issuer:   static.IssuerMonitoring,
		Audience: jwt.Audience{static.AudienceLocate},
	}
	tc, err := controller.NewTokenController(verifier, true, exp)
	rtx.Must(err, "Failed to create token controller")
	monitoringChain := alice.New(tc.Limit).Then(http.HandlerFunc(c.Monitoring))

	// TODO: add verifier for heartbeat access tokens.
	// TODO: add verifier for optional access tokens to support NextRequest.

	mux := http.NewServeMux()
	// Services report their health to the heartbeat service.
	mux.HandleFunc("/v2/heartbeat/", http.HandlerFunc(c.Heartbeat))
	// End to end monitoring requests access tokens for specific targets.
	mux.Handle("/v2/monitoring/", monitoringChain)
	// Clients request access tokens for specific services.
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
