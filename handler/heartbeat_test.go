package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m-lab/locate/clientgeo"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

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

func fakeClient() *Client {
	return NewClient("mlab-sandbox", &fakeSigner{}, &fakeLocator{}, &fakeLocatorV2{},
		clientgeo.NewAppEngineLocator(), prom.NewAPI(nil))
}
