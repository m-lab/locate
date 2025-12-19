// Package locate implements a client for the Locate API v2.
package locate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"

	v2 "github.com/m-lab/locate/api/v2"
)

func TestClient_Nearest(t *testing.T) {
	tests := []struct {
		name          string
		UserAgent     string
		service       string
		BaseURL       *url.URL
		authorization string // optional authorization
		reply         *v2.NearestResult
		status        int
		wantErr       bool
		closeServer   bool
		badJSON       bool
	}{
		{
			name:      "success w/o authorization",
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
			name:          "success w/ authorization",
			service:       "ndt/ndt7",
			UserAgent:     "unit-test",
			authorization: "thisIsMyToken",
			status:        http.StatusOK,
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
			var (
				seenAuth string
				mu       = &sync.Mutex{}
			)
			c := NewClient(tt.UserAgent)
			c.Authorization = tt.authorization // optional
			mux := http.NewServeMux()
			path := "/v2/nearest/" + tt.service
			mux.HandleFunc(path, func(rw http.ResponseWriter, req *http.Request) {
				// save the authorization header for later checking
				mu.Lock()
				seenAuth = req.Header.Get("Authorization")
				mu.Unlock()

				// write configured response
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
				t.Fatalf("Client.Nearest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.reply != nil && !reflect.DeepEqual(got, tt.reply.Results) {
				t.Fatalf("Client.Nearest() = %v, want %v", got, tt.reply.Results)
			}

			// check for authorization
			var expectAuth string
			if tt.authorization != "" {
				expectAuth = fmt.Sprintf("Bearer %s", tt.authorization)
			}
			mu.Lock()
			gotAuth := seenAuth
			mu.Unlock()
			if expectAuth != gotAuth {
				t.Fatalf("auth expect = %v, got = %v", expectAuth, gotAuth)
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
