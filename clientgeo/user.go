package clientgeo

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/m-lab/locate/static"
)

// UserLocator definition for accepting user provided location hints.
type UserLocator struct{}

// Error values returned by Locate.
var (
	ErrNoUserParameters       = errors.New("no user location parameters provided")
	ErrUnusableUserParameters = errors.New("user provided location parameters were unusable")
)

// NewUserLocator creates a new UserLocator.
func NewUserLocator() *UserLocator {
	return &UserLocator{}
}

// Locate looks for user-provided parameters to specify the client location.
func (u *UserLocator) Locate(req *http.Request) (*Location, error) {
	lat := req.URL.Query().Get("lat")
	lon := req.URL.Query().Get("lon")
	if lat != "" && lon != "" {
		// Verify that these are valid floating values.
		flat, errLat := strconv.ParseFloat(lat, 64)
		flon, errLon := strconv.ParseFloat(lon, 64)
		if errLat != nil || errLon != nil ||
			math.IsNaN(flat) || math.IsInf(flat, 0) ||
			math.IsNaN(flon) || math.IsInf(flon, 0) ||
			-90 > flat || flat > 90 ||
			-180 > flon || flon > 180 {
			return nil, ErrUnusableUserParameters
		}
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
