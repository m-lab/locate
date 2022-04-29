package connection

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/static"
)

func Test_Dial(t *testing.T) {
	c := NewConn()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer close(c, s)

	err := c.Dial(s.URL, http.Header{})

	if err != nil {
		t.Errorf("Dial() should have returned nil error, err: %v", err)
	}

	if c.ws == nil {
		t.Error("Dial() error, websocket is nil")
	}

	if !c.IsConnected() {
		t.Error("Dial() error, not connected")
	}
}

func Test_Dial_InvalidUrl(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "malformed",
			url:  "foo",
		},
		{
			name: "https",
			url:  "https://127.0.0.1:46311/v2/heartbeat/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConn()
			err := c.Dial(tt.url, http.Header{})

			if err == nil {
				t.Error("Dial() should return an error when given an invalid URL")
			}
		})
	}
}

func Test_Dial_ServerDown(t *testing.T) {
	c := NewConn()
	defer c.Close()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	// Shut down server for testing.
	s.Close()

	c.InitialInterval = 500 * time.Millisecond
	c.MaxElapsedTime = time.Second

	err := c.Dial(s.URL, http.Header{})
	if err == nil {
		t.Error("Dial() should return an error once backoff ticker stops")
	}
}

func Test_WriteMessage(t *testing.T) {
	tests := []struct {
		name       string
		disconnect bool
		wantErr    bool
	}{
		{
			name:       "success",
			disconnect: false,
			wantErr:    false,
		},
		{
			name:       "disconnect-reconnect",
			disconnect: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConn()
			fh := testdata.FakeHandler{}
			s := testdata.FakeServer(fh.Upgrade)

			c.Dial(s.URL, http.Header{})

			if tt.disconnect {
				fh.Close()
			}

			// Write new message. If connection was closed, it should reconnect
			// and return a write error.
			err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))

			if err == nil && tt.wantErr {
				t.Error("Writing after a disconnect should return an error, close, and reconnect")
			}

			// Connection should be alive and write should succeed.
			err = c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
			if err != nil {
				t.Errorf("WriteMessage() should have succeeded; err: %v", err)
			}

			close(c, s)
		})
	}
}

func Test_CloseAndReconnect(t *testing.T) {
	c := NewConn()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer close(c, s)
	// For testing, make this time window smaller.
	c.MaxReconnectionsTime = 2 * time.Second
	c.Dial(s.URL, http.Header{})

	for i := 0; i < static.MaxReconnectionsTotal; i++ {
		fh.Close()

		err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
		if err == nil {
			t.Error("Writing after a disconnect should return an error, close, and reconnect")
		}

		if !c.IsConnected() {
			t.Error("WriteMessage() failed to reconnect")
		}

		err = c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
		if err != nil {
			t.Errorf("WriteMessage() should succeed after reconnection; err: %v", err)
		}
	}

	// It should not reconnect once it reaches the maximum number of attempts.
	fh.Close()
	err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if err == nil {
		t.Error("WriteMessage() should fail while disconnected")
	}
	if c.IsConnected() {
		t.Error("WriteMessage() reconnection should fail after MaxReconnectionsTotal reached")
	}

	// It should reconnect again after calling WriteMessage once the number of reconnections
	// is reset.
	timer := time.NewTimer(2 * c.MaxReconnectionsTime)
	<-timer.C
	c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if !c.IsConnected() {
		t.Error("WriteMessage() failed to reconnect after MaxReconnectionsTime")
	}
	err = c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if err != nil {
		t.Errorf("WriteMessage() should succeed after reconnection; err: %v",
			err)
	}
}

func close(c *Conn, s *httptest.Server) {
	c.Close()
	s.Close()
}
