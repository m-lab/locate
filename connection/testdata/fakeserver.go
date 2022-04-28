package testdata

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gorilla/websocket"
)

func FakeServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/v2/heartbeat/", http.HandlerFunc(fakeHandler))
	s := httptest.NewServer(mux)
	s.URL = strings.Replace(s.URL, "http", "ws", 1) + "/v2/heartbeat/"
	return s
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
