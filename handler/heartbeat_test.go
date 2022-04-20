package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/clientgeo"
)

func TestClient_Heartbeat(t *testing.T) {
	tests := []struct {
		name     string
		scheme   string
		wantErr  bool
		wantCode int
	}{
		{
			name:     "success",
			scheme:   "ws",
			wantErr:  false,
			wantCode: http.StatusSwitchingProtocols,
		},
		{
			name:    "bad-request",
			scheme:  "https",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator())
			s := httptest.NewServer(http.HandlerFunc(c.Heartbeat))
			defer s.Close()

			URL, _ := url.Parse(s.URL)
			URL.Scheme = tt.scheme
			headers := http.Header{}
			ctx := context.Background()
			dialer := websocket.Dialer{}
			ws, resp, err := dialer.DialContext(ctx, URL.String(), headers)
			if ws != nil {
				defer ws.Close()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Heartbeat() connection error; got: %v, want: %v", err, tt.wantErr)
			}

			if resp != nil && resp.StatusCode != tt.wantCode {
				t.Errorf("Heartbeat() failed to switch protocol; got,%d, want %d", resp.StatusCode, tt.wantCode)
			}
		})
	}
}
