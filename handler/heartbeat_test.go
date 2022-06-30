package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/instances"
	"github.com/m-lab/locate/instances/instancestest"
)

func init() {
	readDeadline = 50 * time.Millisecond
}

func mustSetupTest(t testing.TB) (*Client, *websocket.Conn, func(tb testing.TB)) {
	c := fakeClient()
	s := httptest.NewServer(http.HandlerFunc(c.Heartbeat))
	u, _ := url.Parse(s.URL)
	u.Scheme = "ws"
	dialer := websocket.Dialer{}
	ws, _, err := dialer.Dial(u.String(), http.Header{})
	if err != nil {
		s.Close()
		t.Fatalf("failed to establish a connection, err: %v", err)
	}

	return c, ws, func(t testing.TB) {
		s.Close()
		ws.Close()
	}
}

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

func TestClient_Heartbeat_Timeout(t *testing.T) {
	_, _, teardown := mustSetupTest(t)
	defer teardown(t)

	// The read loop runs in a separate goroutine, so we wait for it to exit.
	// It will exit if there are no new messages within the read deadline.
	before := runtime.NumGoroutine()
	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	after := runtime.NumGoroutine()
	if after >= before {
		t.Errorf("Heartbeat() goroutine did not exit; got: %d, want: <%d", after, before)
	}
}

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator(),
		instances.NewCachingInstanceHandler(&instancestest.FakeDatastoreClient{}))
}
