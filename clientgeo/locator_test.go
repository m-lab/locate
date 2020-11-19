// Package clientgeo supports interfaces to different data sources to help
// identify client geo location for server selection.
package clientgeo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNullLocator_Locate(t *testing.T) {
	nl := &NullLocator{}
	req := httptest.NewRequest(http.MethodGet, "/anyurl", nil)
	nl.Reload(context.Background())
	l, err := nl.Locate(req)
	if err != nil {
		t.Fatalf("NullLocator.Locate return an err; %v", err)
	}
	if l.Latitude != "0.000000" && l.Longitude != "0.000000" {
		t.Fatalf("NullLocator.Location has wrong values; want: 0.000000,0.000000 got: %#v", l)
	}
}
