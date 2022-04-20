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

func TestClient_Heartbeat_Error(t *testing.T) {
	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v2/heartbeat/", nil)
	req.RemoteAddr = "not-an-ip"
	c := fakeClient()
	c.Heartbeat(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("Heartbeat() wrong status code; got %d, want %d", rw.Code, http.StatusBadRequest)
	}
}

func TestClient_Heartbeat_Success(t *testing.T) {
	c := fakeClient()
	s := httptest.NewServer(http.HandlerFunc(c.Heartbeat))
	defer s.Close()

	URL, _ := url.Parse(s.URL)
	URL.Scheme = "ws"
	headers := http.Header{}
	ctx := context.Background()
	dialer := websocket.Dialer{}
	ws, resp, err := dialer.DialContext(ctx, URL.String(), headers)
	defer ws.Close()

	if err != nil {
		t.Errorf("Heartbeat() connection error; got: %v, want: %v", err, nil)
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("Heartbeat() failed to switch protocol; got,%d, want %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}

	if err = ws.WriteMessage(1, []byte("Incoming")); err != nil {
		t.Errorf("Heartbeat() could not write message to connection; err: %v", err)
	}
}

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator())
}
