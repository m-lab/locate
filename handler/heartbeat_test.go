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
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/connection/testdata"
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
		c.instances = make(map[string]*v2.HeartbeatMessage)
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

func TestClient_Heartbeat_Success(t *testing.T) {
	c, ws, teardown := mustSetupTest(t)
	defer teardown(t)

	err := ws.WriteJSON(testdata.FakeRegistration)
	testingx.Must(t, err, "failed to write registration message")
	err = ws.WriteJSON(testdata.FakeHealth)
	testingx.Must(t, err, "failed to write health message")

	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	c.mu.RLock()
	defer c.mu.RUnlock()
	val := c.instances[testdata.FakeHostname]
	if diff := deep.Equal(val.Registration, testdata.FakeRegistration.Registration); diff != nil {
		t.Errorf("Heartbeat() did not save instance information; got: %v, want: %v",
			val.Registration, *testdata.FakeRegistration.Registration)
	}
	if diff := deep.Equal(val.Health, testdata.FakeHealth.Health); diff != nil {
		t.Errorf("Heartbeat() did not update health; got: %v, want: %v",
			val.Health, testdata.FakeHealth.Health)
	}

}

func TestClient_Heartbeat_InvalidRegistrationType(t *testing.T) {
	c, ws, teardown := mustSetupTest(t)
	defer teardown(t)

	type test struct {
		Foo string
	}
	err := ws.WriteJSON(test{Foo: "bar"})
	testingx.Must(t, err, "failed to write invalid registration message")

	timer := time.NewTimer(2 * readDeadline)
	<-timer.C
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.instances) > 0 {
		t.Errorf("Heartbeat() expected instances to be empty")
	}
}

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, clientgeo.NewAppEngineLocator())
}
