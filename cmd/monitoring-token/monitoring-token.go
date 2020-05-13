package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/m-lab/access/token"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/logx"
	"github.com/m-lab/go/pretty"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/proxy"
	"github.com/m-lab/locate/static"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	locate    = flagx.MustNewURL("http://localhost:8080/v2/monitoring/")
	privKey   flagx.FileBytes
	machine   string
	service   string
	timeout   time.Duration
	envName   string
	envValue  string
	logFatalf = log.Fatalf
)

func init() {
	setupFlags()
}

func setupFlags() {
	flag.Var(&locate, "locate-url", "URL Prefix for locate service")
	flag.Var(&privKey, "monitoring-signer-key", "Private JWT key used for signing")
	flag.StringVar(&machine, "machine", "", "Machine name used as Audience in the jwt Claim")
	flag.StringVar(&service, "service", "ndt/ndt5", "<experiment>/<datatype> to request monitoring access tokens")
	flag.DurationVar(&timeout, "timeout", 60*time.Second, "Complete request and command execution within timeout")
	flag.StringVar(&envName, "env-name", "SERVICE_URL", "Export the access token to the named environment variable before executing given command")
	flag.StringVar(&envValue, "env-value", "wss://:3010/ndt_protocol", "The key name to extract form the monitoring result Target.URLs")
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnvWithLog(flag.CommandLine, false), "Failed to read args from env")

	// This process signs access tokens for /v2/monitoring requests to the
	// locate service. NOTE: the locate service MUST be configured with the
	// corresponding public key to verify these access tokens.
	priv, err := token.NewSigner(privKey)
	rtx.Must(err, "Failed to allocate signer")

	// Create a claim, similar to the locate service, and sign it.
	cl := jwt.Claims{
		Issuer:   static.IssuerMonitoring,
		Subject:  machine,
		Audience: jwt.Audience{static.AudienceLocate},
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}

	// Signing the claim generates the access token string.
	logx.Debug.Println(cl)
	token, err := priv.Sign(cl)
	rtx.Must(err, "Failed to sign claims")

	// Add the token to the URL parameters in the request to the locate service.
	params := url.Values{}
	params.Set("access_token", token)
	locate.RawQuery = params.Encode()
	locate.Path = locate.Path + service

	// Prepare a context with absolute timeout for getting token and running command.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	logx.Debug.Println("Issue request to:", locate.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, locate.String(), nil)
	rtx.Must(err, "Failed to create request from url: %q", locate.URL)

	// Get monitoring result.
	mr := &v2.MonitoringResult{}
	_, err = proxy.UnmarshalResponse(req, mr)
	rtx.Must(err, "Failed to get response")
	logx.Debug.Println(pretty.Sprint(mr))
	if mr.Error != nil {
		logFatalf("ERROR: %s %s", mr.Error.Title, mr.Error.Detail)
		return
	}
	if mr.Target == nil {
		logFatalf("ERROR: monitoring result Target field is nil!")
		return
	}

	// Place the URL into the named environment variable for access by the command.
	os.Setenv(envName, mr.Target.URLs[envValue])
	logx.Debug.Println("Setting:", envName, "=", mr.Target.URLs[envValue])
	logx.Debug.Println("Exec:", flag.Args())
	args := flag.Args()
	if len(args) == 0 {
		logFatalf("ERROR: no command given to execute")
		return
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if logx.LogxDebug.Get() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	rtx.Must(cmd.Run(), "Failed to run %#v", args)
}
