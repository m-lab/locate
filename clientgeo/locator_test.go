// Package clientgeo supports interfaces to different data sources to help
// identify client geo location for server selection.
package clientgeo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
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

type errLocator struct{}

func (e *errLocator) Locate(req *http.Request) (*Location, error) {
	return nil, errors.New("error")
}

func (e *errLocator) Reload(ctx context.Context) {}

func TestMultiLocator(t *testing.T) {
	want := &Location{
		Latitude:  "0.000000",
		Longitude: "0.000000",
	}
	t.Run("success", func(t *testing.T) {
		ml := MultiLocator{&errLocator{}, &NullLocator{}}
		req := httptest.NewRequest(http.MethodGet, "/anyurl", nil)
		l, err := ml.Locate(req)
		if err != nil {
			t.Errorf("MultiLocator.Locate returned error: %v", err)
		}
		if !reflect.DeepEqual(l, want) {
			t.Errorf("MultiLocator() = %v, want %v", l, want)
		}
		ml.Reload(req.Context())
	})
	t.Run("all-errors", func(t *testing.T) {
		ml := MultiLocator{&errLocator{}, &errLocator{}}
		req := httptest.NewRequest(http.MethodGet, "/anyurl", nil)
		_, err := ml.Locate(req)
		if err == nil {
			t.Errorf("MultiLocator.Locate should return error: got nil")
		}
	})
}
