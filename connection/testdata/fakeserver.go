package testdata

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

func FakeServer(handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/v2/heartbeat", http.HandlerFunc(handler))
	s := httptest.NewServer(mux)
	s.URL = strings.Replace(s.URL, "http", "ws", 1) + "/v2/heartbeat"
	return s
}

type FakeHandler struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (fh *FakeHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	upgrader := websocket.Upgrader{}
	fh.conn, _ = upgrader.Upgrade(w, r, nil)
}

func (fh *FakeHandler) BadUpgrade(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(400)
}

func (fh *FakeHandler) Read() ([]byte, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	_, msg, err := fh.conn.ReadMessage()
	return msg, err
}

func (fh *FakeHandler) Close() {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	fh.conn.Close()
}
