// Package clientgeo supports interfaces to different data sources to help
// identify client geo location for server selection.
package clientgeo

import (
	"context"
	"net/http"
)

// Locator supports locating a client request and Reloading the underlying database.
type Locator interface {
	Locate(req *http.Request) (*Location, error)
	Reload(context.Context)
}

// Location contains an estimated the latitude and longitude of a client IP.
type Location struct {
	Latitude  string
	Longitude string
	Headers   http.Header
}

// NullLocator always returns a client location of 0,0.
type NullLocator struct{}

// Locate returns the static 0,0 lat/lon location.
func (f *NullLocator) Locate(req *http.Request) (*Location, error) {
	return &Location{
		Latitude:  "0.000000",
		Longitude: "0.000000",
	}, nil
}

// Reload does nothing.
func (f *NullLocator) Reload(ctx context.Context) {}
