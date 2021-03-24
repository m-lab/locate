package main

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/m-lab/go/logx"
	"github.com/m-lab/go/rtx"
)

var (
	insecurePrivateKey = `{"use":"sig","kty":"OKP","kid":"insecure","crv":"Ed25519","alg":"EdDSA","x":"E50_cwU7ACoH_XM6We3AFLHVWA63xm2crFhKL-PUc3Y","d":"3JRzWpk6aILrhOnry41Fu3u9l0XbloAVhuVNowWqT_Y"}`
	insecurePublicKey  = `{"use":"sig","kty":"OKP","kid":"insecure","crv":"Ed25519","alg":"EdDSA","x":"E50_cwU7ACoH_XM6We3AFLHVWA63xm2crFhKL-PUc3Y"}`
)

func Test_main(t *testing.T) {
	tests := []struct {
		name string
		resp string
		args []string
	}{
		{
			name: "error-service-url-error-not-nil",
			resp: `{"error":{"type":"fake-error"}}`,
			args: []string{"monitoring-token", "-service-url"},
		},
		{
			name: "error-service-url-nil-target",
			resp: `{}`,
			args: []string{"monitoring-token", "-service-url"},
		},
		{
			name: "error-service-url-args-target-len-zero",
			resp: `{"target": {"urls": {"wss://:3010/ndt_protocol":""}}}`,
			args: []string{"monitoring-token", "-service-url"},
		},
		{
			name: "success-service-url-value-matches-check_env-arg",
			// The value "FAKE_URL" is provided to the check_env.sh command via the environment.
			// The argument to check_env.sh is the value it expects in the SERVICE_URL env variable.
			resp: `{"target": {"urls": {"wss://:3010/ndt_protocol":"FAKE_VALUE"}}}`,
			args: []string{"monitoring-token", "-service-url", "-env-name=SVC_URL", "--", "testdata/check_env.sh", "SVC_URL", "FAKE_VALUE"},
		},
		{
			name: "success-locate-monitoring-url",
			resp: `{}`,
			args: []string{"monitoring-token"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Completely reset command line flags, since main uses these to run a command.
			os.Args = tt.args
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			if strings.Contains(tt.name, "success") {
				// Enable debug logging on the success case to visit the
				// stdout/stderr path when executing the command.
				logx.LogxDebug.Set("true")
			}
			setupFlags()

			logFatalf = func(format string, v ...interface{}) {}
			handler := func(rw http.ResponseWriter, req *http.Request) {
				rw.Write([]byte(tt.resp))
			}
			privKey = []byte(insecurePrivateKey)
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/monitoring/", handler)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			var err error
			locate.URL, err = url.Parse(srv.URL + "/v2/monitoring/")
			rtx.Must(err, "failed to parse url: %q", srv.URL)

			main()
		})
	}
}
