package connection

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/static"
)

func Test_Dial(t *testing.T) {
	c := NewConn()
	s := fakeServer()
	defer close(c, s)

	c.Dial(s.URL, http.Header{})

	if c.ws == nil {
		t.Error("Dial() error, websocket is nil")
	}

	if !c.IsConnected() {
		t.Error("Dial() error, not connected")
	}
}

func Test_WriteMessage(t *testing.T) {
	tests := []struct {
		name       string
		disconnect bool
		wantErr    error
	}{
		{
			name:       "success",
			disconnect: false,
			wantErr:    nil,
		},
		{
			name:       "disconnect-reconnect",
			disconnect: true,
			wantErr:    ErrNotConnected,
		},
	}

	for _, tt := range tests {
		c := NewConn()
		s := fakeServer()

		c.Dial(s.URL, http.Header{})

		if tt.disconnect {
			c.Close()
		}

		// Write new message. If connection was closed, it should reconnect
		// and return a write error.
		err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))

		if err != tt.wantErr {
			t.Errorf("WriteMessage() error; got: %v, want: %v", err, tt.wantErr)
		}

		// Connection should be alive and write should succeed.
		err = c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
		if err != nil {
			t.Errorf("WriteMessage() should have succeeded; err: %v", err)
		}

		close(c, s)
	}
}

func Test_CloseAndReconnect(t *testing.T) {
	c := NewConn()
	s := fakeServer()
	defer close(c, s)
	c.Dial(s.URL, http.Header{})

	for i := 0; i < static.MaxReconnectionsTotal; i++ {
		c.Close()
		if c.IsConnected() {
			t.Errorf("Close() failed to close connection")
		}

		c.reconnect()
		if !c.IsConnected() {
			t.Errorf("reconnect() failed to reconnect")
		}

		err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
		if err != nil {
			t.Errorf("WriteMessage() should succeed after reconnection; err: %v", err)
		}
	}

	c.Close()
	c.reconnect()
	if c.IsConnected() {
		t.Errorf("reconnect() should fail after maximum reconnections reached")
	}
	err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if err == nil {
		t.Errorf("WriteMessage() should fail while disconnected")
	}
}

func fakeHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			return
		}
	}
}

func fakeServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/v2/heartbeat/", http.HandlerFunc(fakeHandler))
	s := httptest.NewServer(mux)
	s.URL = strings.Replace(s.URL, "http", "ws", 1) + "/v2/heartbeat/"
	return s
}

func close(c *Conn, s *httptest.Server) {
	c.Close()
	s.Close()
}
