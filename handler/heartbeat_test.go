package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestClient_Heartbeat_Error(t *testing.T) {
	rw := httptest.NewRecorder()
	// The header from this request will not contain the
	// necessary "upgrade" tokens.
	req := httptest.NewRequest(http.MethodGet, "/v2/heartbeat", nil)
	c := fakeClient(nil)
	c.Heartbeat(rw, req)

	if rw.Code != http.StatusBadRequest {
		t.Errorf("Heartbeat() wrong status code; got %d, want %d", rw.Code, http.StatusBadRequest)
	}
}

func TestClient_handleHeartbeats(t *testing.T) {
	wantErr := errors.New("connection error")
	tests := []struct {
		name    string
		ws      conn
		tracker heartbeat.StatusTracker
	}{
		{
			name: "read-err",
			ws: &fakeConn{
				err: wantErr,
			},
		},
		{
			name: "registration-err",
			ws: &fakeConn{
				msg: testdata.FakeRegistration,
			},
			tracker: &heartbeattest.FakeStatusTracker{Err: wantErr},
		},
		{
			name: "health-err",
			ws: &fakeConn{
				msg: testdata.FakeHealth,
			},
			tracker: &heartbeattest.FakeStatusTracker{Err: wantErr},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fakeClient(tt.tracker)
			err := c.handleHeartbeats(tt.ws)
			if !errors.Is(err, wantErr) {
				t.Errorf("Client.handleHeartbeats() error = %v, wantErr %v", err, wantErr)
			}
		})
	}
}

func fakeClient(t heartbeat.StatusTracker) *Client {
	locatorv2 := fakeLocatorV2{StatusTracker: t}
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, &locatorv2,
		clientgeo.NewAppEngineLocator(), prom.NewAPI(nil))
}

type fakeConn struct {
	msg any
	err error
}

// ReadMessage returns 0, the JSON encoding of a fake message, and an error.
func (c *fakeConn) ReadMessage() (int, []byte, error) {
	jsonMsg, _ := json.Marshal(c.msg)
	return 0, jsonMsg, c.err
}

// SetReadDeadline returns nil.
func (c *fakeConn) SetReadDeadline(time.Time) error {
	return nil
}

// Close returns nil.
func (c *fakeConn) Close() error {
	return nil
}
