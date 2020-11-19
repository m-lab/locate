// Package clientgeo supports interfaces to different data sources to help
// identify client geo location for server selection.
package clientgeo

import (
	"context"
	"net/http"

	"github.com/hashicorp/go-multierror"
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

// MultiLocator wraps several Locator types into the Locate interface.
type MultiLocator []Locator

// NewMultiLocator creates a new Locator from all individual parameters.
func NewMultiLocator(locators ...Locator) MultiLocator {
	return locators
}

// Locate calls Locate on all client Locators. The first successfully identifiec
// location is returned. If all Locators returns an error, a multierror.Error is
// returned as an error with all Locator error messages.
func (g MultiLocator) Locate(req *http.Request) (*Location, error) {
	var merr *multierror.Error
	for _, locator := range g {
		l, err := locator.Locate(req)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		return l, nil
	}
	return nil, merr
}

// Reload calls Reload on all Client Locators.
func (g MultiLocator) Reload(ctx context.Context) {
	for _, locator := range g {
		locator.Reload(ctx)
	}
}
