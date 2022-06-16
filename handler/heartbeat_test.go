package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/connection/testdata"
)

func init() {
	readDeadline = 50 * time.Millisecond
}

func setupTest(t testing.TB) (*Client, *websocket.Conn, error, func(tb testing.TB)) {
	c := fakeClient()
	s := httptest.NewServer(http.HandlerFunc(c.Heartbeat))
	u, _ := url.Parse(s.URL)
	u.Scheme = "ws"
	dialer := websocket.Dialer{}
	ws, _, err := dialer.Dial(u.String(), http.Header{})

	return c, ws, err, func(t testing.TB) {
		s.Close()
		ws.Close()
		c.instances = make(map[string]*instanceData)
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
	_, _, err, teardown := setupTest(t)
	defer teardown(t)

	if err != nil {
		t.Fatalf("Heartbeat() connection error; got: %v, want: %v", err, nil)
	}

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

func TestClient_Heartbeat_Success(t *testing.T) {
	c, ws, _, teardown := setupTest(t)
	defer teardown(t)

	ws.WriteMessage(1, testdata.EncodedRegistration)
	ws.WriteMessage(1, testdata.EncodedHealth)

	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, _ := c.instances[testdata.FakeHostname]
	if diff := deep.Equal(testdata.FakeRegistration, val.instance); diff != nil {
		t.Errorf("Heartbeat() did not save instance information; got: %v, want: %v",
			val.instance, testdata.FakeRegistration)
	}
	if val.health != testdata.FakeHealth.Score {
		t.Errorf("Heartbeat() did not update health score; got: %f, want: %f",
			val.health, testdata.FakeHealth.Score)
	}
}

func TestClient_Heartbeat_InvalidRegistration(t *testing.T) {
	c, ws, _, teardown := setupTest(t)
	defer teardown(t)

	ws.WriteMessage(1, []byte("foo"))

	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.instances) > 0 {
		t.Errorf("Heartbeat() expected instances to be empty")
	}
}

func TestClient_Heartbeat_InvalidHealth(t *testing.T) {
	c, ws, _, teardown := setupTest(t)
	defer teardown(t)

	ws.WriteMessage(1, testdata.EncodedRegistration)
	ws.WriteMessage(1, []byte("foo"))

	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, _ := c.instances[testdata.FakeHostname]
	if val.health != 0 {
		t.Errorf("Heartbeat() should not have updated the health score; got: %f, want: 0",
			val.health)
	}
}

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator())
}
