package testdata

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/websocket"
)

func FakeServer(handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/v2/heartbeat/", http.HandlerFunc(handler))
	s := httptest.NewServer(mux)
	s.URL = strings.Replace(s.URL, "http", "ws", 1) + "/v2/heartbeat/"
	return s
}

type FakeHandler struct {
	conn *websocket.Conn
}

func (fh *FakeHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	fh.conn, _ = upgrader.Upgrade(w, r, nil)
}

func (fh *FakeHandler) Read() ([]byte, error) {
	_, msg, err := fh.conn.ReadMessage()
	return msg, err
}

func (fh *FakeHandler) Close() {
	fh.conn.Close()
}
