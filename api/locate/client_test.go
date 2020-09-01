// Package locate implements a client for the Locate API v2.
package locate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
)

func mustParseURL(u string) *url.URL {
	URL, err := url.Parse(u)
	rtx.Must(err, "failed to parse url")
	return URL
}

func TestClient_Nearest(t *testing.T) {
	tests := []struct {
		name        string
		UserAgent   string
		service     string
		BaseURL     *url.URL
		reply       *v2.NearestResult
		status      int
		wantErr     bool
		closeServer bool
		badJSON     bool
	}{
		/*
		 */
		{
			name:      "success",
			service:   "ndt/ndt7",
			UserAgent: "unit-test",
			status:    http.StatusOK,
			reply: &v2.NearestResult{
				Results: []v2.Target{
					{
						Machine: "mlab1-foo01.mlab-sandbox.measurement-lab.org",
						URLs: map[string]string{
							"ws:///ndt/v7/download": "fake-url"},
					},
				},
			},
		},
		{
			name:      "error-nil-results",
			service:   "ndt/ndt7",
			UserAgent: "unit-test",
			status:    http.StatusOK,
			reply:     &v2.NearestResult{},
			wantErr:   true,
		},
		{
			name:      "error-empty-user-agent",
			service:   "ndt/ndt7",
			UserAgent: "", // empty user agent.
			reply:     &v2.NearestResult{},
			wantErr:   true,
		},
		{
			name:        "error-http-client-do-failure",
			service:     "ndt/ndt7",
			UserAgent:   "fake-user-agent",
			reply:       &v2.NearestResult{},
			wantErr:     true,
			closeServer: true,
		},
		{
			name:      "error-bad-json-response",
			service:   "ndt/ndt7",
			UserAgent: "fake-user-agent",
			status:    http.StatusOK,
			wantErr:   true,
			badJSON:   true,
		},
		{
			name:      "error-result-with-error",
			service:   "ndt/ndt7",
			UserAgent: "unit-test",
			status:    http.StatusInternalServerError,
			reply: &v2.NearestResult{
				Error: &v2.Error{
					Title:  "error",
					Detail: "detail",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.UserAgent)
			mux := http.NewServeMux()
			path := "/v2/nearest/" + tt.service
			mux.HandleFunc(path, func(rw http.ResponseWriter, req *http.Request) {
				var b []byte
				if tt.badJSON {
					b = []byte("this-is-not-JSON{")
				} else {
					b, _ = json.Marshal(tt.reply)
				}
				rw.WriteHeader(tt.status)
				rw.Write(b)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			if tt.closeServer {
				srv.Close()
			}

			u, err := url.Parse(srv.URL)
			u.Path = "/v2/nearest"
			if tt.BaseURL != nil {
				c.BaseURL = tt.BaseURL
			} else {
				c.BaseURL = u
			}

			ctx := context.Background()
			got, err := c.Nearest(ctx, tt.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Nearest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.reply != nil && !reflect.DeepEqual(got, tt.reply.Results) {
				t.Errorf("Client.Nearest() = %v, want %v", got, tt.reply.Results)
			}
		})
	}
}

// This test case is unreachable through the public interface.
func TestClient_get(t *testing.T) {
	t.Run("bad-url", func(t *testing.T) {
		c := &Client{}
		ctx := context.Background()
		_, _, err := c.get(ctx, "://this-is-invalid")
		if err == nil {
			t.Errorf("Client.get() error nil, wantErr")
			return
		}
	})
}
