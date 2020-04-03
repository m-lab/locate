package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	kms "cloud.google.com/go/kms/apiv1"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/handler"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/signer"
)

var (
	listenPort string
	project    string
	verifyKey  flagx.FileBytes
)

func init() {
	// PORT and GOOGLE_CLOUD_PROJECT are part of the default App Engine environment.
	flag.StringVar(&listenPort, "port", "8080", "AppEngine port environment variable")
	flag.StringVar(&project, "google-cloud-project", "", "AppEngine project environment variable")
	flag.Var(&verifyKey, "verify-key", "")
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	client, err := kms.NewKeyManagementClient(mainCtx)
	rtx.Must(err, "Failed to create KMS client")
	cfg := signer.NewConfig(project, "global", "locate-signer", "private-jwk")
	// Load encrypted signer key from environment, using variable name derived from project.
	signer, err := cfg.Load(mainCtx, client, os.Getenv("ENCRYPTED_SIGNER_KEY_"+strings.ReplaceAll(project, "-", "_")))
	rtx.Must(err, "Failed to load signer key")
	locator := proxy.MustNewLegacyLocator(proxy.DefaultLegacyServer)
	c := handler.NewClient(project, signer, locator)

	// TODO: add verifier chain for monitoring requests.
	// TODO: add verifier for optional access tokens to support NextRequest.

	mux := http.NewServeMux()
	// Services report their health to the heartbeat service.
	mux.HandleFunc("/v2/heartbeat/", http.HandlerFunc(c.Heartbeat))
	// End to end monitoring requests access tokens for specific targets.
	mux.Handle("/v2/monitoring/", http.HandlerFunc(c.Monitoring))
	// Clients request access tokens for specific services.
	mux.HandleFunc("/v2/query/", http.HandlerFunc(c.TranslatedQuery))

	srv := &http.Server{
		Addr:    ":" + listenPort,
		Handler: mux,
	}
	log.Println("Listening for INSECURE access requests on " + listenPort)
	rtx.Must(httpx.ListenAndServeAsync(srv), "Could not start server")
	defer srv.Close()
	<-mainCtx.Done()
}
