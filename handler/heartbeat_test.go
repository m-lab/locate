package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/clientgeo"
)

func TestClient_Heartbeat_Error(t *testing.T) {
	rw := httptest.NewRecorder()
	// The header from this request will not contain the
	// necessary "upgrade" tokens.
	req := httptest.NewRequest(http.MethodGet, "/v2/heartbeat", nil)
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

	u, _ := url.Parse(s.URL)
	u.Scheme = "ws"
	ctx := context.Background()
	dialer := websocket.Dialer{}
	readDeadline = 2 * time.Second
	ws, resp, err := dialer.DialContext(ctx, u.String(), http.Header{})

	if err != nil {
		t.Fatalf("Heartbeat() connection error; got: %v, want: %v", err, nil)
	}

	defer ws.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("Heartbeat() failed to switch protocol; got: %d, want: %d", resp.StatusCode, http.StatusSwitchingProtocols)
	}

	if err = ws.WriteMessage(1, []byte("Incoming")); err != nil {
		t.Errorf("Heartbeat() could not write message to connection; err: %v", err)
	}

	// goroutine should exit if there are no new messages within read deadline.
	before := runtime.NumGoroutine()
	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	after := runtime.NumGoroutine()
	if after >= before {
		t.Errorf("Heartbeat() goroutine did not exit; got: %d, want: <%d", after, before)
	}
}

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator())
}
