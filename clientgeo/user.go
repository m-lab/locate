package clientgeo

import (
	"context"
	"errors"
	"net/http"

	"github.com/m-lab/locate/static"
)

type UserLocator struct{}

var ErrNoUserParameters = errors.New("no user location parameters provided")

func NewUserLocator() *UserLocator {
	return &UserLocator{}
}

// Locate looks for user-provided parameters to specify the client location.
func (u *UserLocator) Locate(req *http.Request) (*Location, error) {
	lat := req.URL.Query().Get("lat")
	lon := req.URL.Query().Get("lon")
	if lat != "" && lon != "" {
		loc := &Location{
			Latitude:  lat,
			Longitude: lon,
			Headers:   http.Header{},
		}
		loc.Headers.Set("X-Locate-ClientLatLon", lat+","+lon)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", "user-latlon")
		return loc, nil
	}
	if ll := static.Regions[req.URL.Query().Get("region")]; ll != "" {
		loc, err := splitLatLon(ll)
		loc.Headers.Set("X-Locate-ClientLatLon", ll)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", "user-region")
		return loc, err
	}
	if ll := static.Countries[req.URL.Query().Get("country")]; ll != "" {
		loc, err := splitLatLon(ll)
		loc.Headers.Set("X-Locate-ClientLatLon", ll)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", "user-country")
		return loc, err
	}
	return nil, ErrNoUserParameters
}

// Reload does nothing.
func (u *UserLocator) Reload(ctx context.Context) {}
