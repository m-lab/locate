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
