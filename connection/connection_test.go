package connection

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/connection/testdata"
)

func Test_Dial(t *testing.T) {
	c := NewConn()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer close(c, s)

	err := c.Dial(s.URL, http.Header{}, testdata.FakeRegistration)

	if err != nil {
		t.Errorf("Dial() should have returned nil error, err: %v", err)
	}

	if !c.IsConnected() {
		t.Error("Dial() error, not connected")
	}
}

func Test_Dial_ThenClose(t *testing.T) {
	c := NewConn()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer s.Close()

	if err := c.Dial(s.URL, http.Header{}, testdata.FakeRegistration); err != nil {
		t.Errorf("Dial() should have returned nil error, err: %v", err)
	}

	if !c.IsConnected() {
		t.Error("Dial() error, not connected")
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close() should have returned nil error, err: %v", err)
	}

	if c.IsConnected() {
		t.Error("Close() error, still connected")
	}

	if err := c.Dial(s.URL, http.Header{}, testdata.FakeRegistration); err != nil {
		t.Errorf("Dial() should have returned nil error, err: %v", err)
	}

	if !c.IsConnected() {
		t.Error("Dial() error, not connected")
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close() should have returned nil error, err: %v", err)
	}

	if c.IsConnected() {
		t.Error("Close() error, still connected")
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
			name: "https-invalid-scheme",
			url:  "https://127.0.0.2:1234/v2/heartbeat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConn()
			err := c.Dial(tt.url, http.Header{}, testdata.FakeRegistration)

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

	err := c.Dial(s.URL, http.Header{}, testdata.FakeRegistration)
	if err == nil {
		t.Error("Dial() should return an error once backoff ticker stops")
	}
}

func Test_Dial_BadRequest(t *testing.T) {
	c := NewConn()
	fh := testdata.FakeHandler{}
	// This handler returns a 400 status code.
	s := testdata.FakeServer(fh.BadUpgrade)
	err := c.Dial(s.URL, http.Header{}, testdata.FakeRegistration)

	if err == nil {
		t.Error("Dial() should fail when a 400 response status code is received")
	}
}

func Test_WriteMessage(t *testing.T) {
	tests := []struct {
		name       string
		disconnect bool
	}{
		{
			name:       "success",
			disconnect: false,
		},
		{
			name:       "disconnect-reconnect",
			disconnect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConn()
			fh := testdata.FakeHandler{}
			s := testdata.FakeServer(fh.Upgrade)

			c.Dial(s.URL, http.Header{}, testdata.FakeRegistration)

			if tt.disconnect {
				fh.Close()
			}

			// Write new message. If connection was closed, it should reconnect
			// and retry to send the message.
			err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))

			if err != nil {
				t.Errorf("WriteMessage() should have succeeded; err: %v", err)
			}

			close(c, s)
		})
	}
}

func Test_WriteMessage_ErrNotDailed(t *testing.T) {
	c := NewConn()
	err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if !errors.Is(err, ErrNotDailed) {
		t.Errorf("WriteMessage() incorrect error; got: %v, want: ErrNotDailed", err)
	}
}

func TestWriteMessage_ClosedServer(t *testing.T) {
	c := NewConn()
	defer c.Close()
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	c.InitialInterval = 500 * time.Millisecond
	c.MaxElapsedTime = time.Second
	c.Dial(s.URL, http.Header{}, testdata.FakeRegistration)

	// Shut down server for testing.
	fh.Close()
	s.Close()

	// Write should fail and connection should become disconnected.
	err := c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if err == nil {
		t.Error("WriteMessage() should fail after server is disconnected")
	}
	if c.IsConnected() {
		t.Errorf("IsConnected() should be false after writing to closed server.")
	}

	// Subsequent writes should fail.
	err = c.WriteMessage(websocket.TextMessage, []byte("Health message!"))
	if err == nil {
		t.Error("WriteMessage() should fail after client detects disconnection")
	}
}

func Test_TokenRefresh(t *testing.T) {
	c := NewConn()
	
	// Track token refresh calls
	refreshCount := 0
	c.SetTokenRefresher(func() (string, error) {
		refreshCount++
		// Return a JWT token that expires in 2 hours (won't need refresh)
		return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjk5OTk5OTk5OTl9.fake-signature", nil
	})
	
	// Set up headers with an expired JWT token
	headers := http.Header{}
	headers.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDA5MzUzMDB9.fake-signature") // Expired in 2020
	c.header = headers
	
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer close(c, s)
	
	// Call refreshJWTIfNeeded directly (since connect() is hard to test)
	err := c.refreshJWTIfNeeded()
	if err != nil {
		t.Errorf("refreshJWTIfNeeded() failed: %v", err)
	}
	
	// Token refresh should have been called due to expired token
	if refreshCount != 1 {
		t.Errorf("Expected 1 token refresh call, got %d", refreshCount)
	}
	
	// Check that the Authorization header was updated
	newAuth := c.header.Get("Authorization")
	expectedAuth := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjk5OTk5OTk5OTl9.fake-signature"
	if newAuth != expectedAuth {
		t.Errorf("Authorization header not updated. Got: %s, Expected: %s", newAuth, expectedAuth)
	}
}

func Test_TokenRefresh_NoRefresher(t *testing.T) {
	c := NewConn()
	// Don't set a token refresher
	
	headers := http.Header{}
	headers.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MDA5MzUzMDB9.fake-signature") // Expired token
	c.header = headers
	
	// Should not fail even with expired token and no refresher
	err := c.refreshJWTIfNeeded()
	if err != nil {
		t.Errorf("refreshJWTIfNeeded() should not fail without refresher: %v", err)
	}
}

func Test_TokenRefresh_ValidToken(t *testing.T) {
	c := NewConn()
	
	refreshCount := 0
	c.SetTokenRefresher(func() (string, error) {
		refreshCount++
		return "new-token", nil
	})
	
	headers := http.Header{}
	// Token that expires far in the future (won't need refresh)
	headers.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjk5OTk5OTk5OTl9.fake-signature")
	c.header = headers
	
	err := c.refreshJWTIfNeeded()
	if err != nil {
		t.Errorf("refreshJWTIfNeeded() failed: %v", err)
	}
	
	// Should not refresh since token is still valid
	if refreshCount != 0 {
		t.Errorf("Expected 0 token refresh calls for valid token, got %d", refreshCount)
	}
}

func close(c *Conn, s *httptest.Server) {
	c.Close()
	s.Close()
}
