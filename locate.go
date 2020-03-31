package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/handler"
)

var (
	listenPort string
	project    string
	signerKey  string
	verifyKey  flagx.FileBytes
)

func init() {
	flag.StringVar(&listenPort, "port", "8080", "AppEngine port environment variable")
	flag.StringVar(&project, "google-cloud-project", "", "AppEngine project environment variable")
	flag.StringVar(&signerKey, "encrypted-signer-key", "", "")
	flag.Var(&verifyKey, "verify-key", "")
}

var mainCtx, mainCancel = context.WithCancel(context.Background())

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Could not parse env args")

	// TODO: load signer key.
	c := handler.NewClient(project, nil)

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
