package healthtest

import (
	"net/http"
	"net/http/httptest"
)

func TestHealthServer(code int) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
	s := httptest.NewServer(mux)
	return s
}
