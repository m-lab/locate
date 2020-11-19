package clientgeo

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/apex/log"
	"github.com/m-lab/locate/static"
)

var (
	// ErrBadLatLonFormat is returned with a lat,lon header is missing or corrupt.
	ErrBadLatLonFormat = errors.New("lat,lon format was missing or corrupt")

	// ErrNullLatLon is returned with a 0,0 lat/lon value is provided.
	ErrNullLatLon = errors.New("lat,lon value was null: " + nullLatLon)

	latlonMethod  = "appengine-latlong"
	regionMethod  = "appengine-region"
	countryMethod = "appengine-country"
	noneMethod    = "appengine-none"
	nullLatLon    = "0.000000,0.000000"
)

// NewAppEngineLocator creates a new AppEngineLocator.
func NewAppEngineLocator() *AppEngineLocator {
	return &AppEngineLocator{}
}

// AppEngineLocator finds a client location using AppEngine headers for lat/lon,
// region, or country.
type AppEngineLocator struct{}

// Locate finds a location for the given client request using AppEngine headers.
// If no location is found, an error is returned.
func (sl *AppEngineLocator) Locate(req *http.Request) (*Location, error) {
	headers := req.Header
	fields := log.Fields{
		"CityLatLong": headers.Get("X-AppEngine-CityLatLong"),
		"Country":     headers.Get("X-AppEngine-Country"),
		"Region":      headers.Get("X-AppEngine-Region"),
		"Proto":       headers.Get("X-Forwarded-Proto"),
		"Path":        req.URL.Path,
	}

	// First, try the given lat/lon. Avoid invalid values like 0,0.
	latlon := headers.Get("X-AppEngine-CityLatLong")
	loc, err := splitLatLon(latlon)
	if err == nil {
		log.WithFields(fields).Info(latlonMethod)
		loc.Headers.Set("X-Locate-ClientLatLon", latlon)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", latlonMethod)
		return loc, nil
	}
	// The next two fallback methods require the country, so check this next.
	country := headers.Get("X-AppEngine-Country")
	if country == "" || static.Countries[country] == "" {
		// Without a valid country value, we can neither lookup the
		// region nor country.
		log.WithFields(fields).Info(noneMethod)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", noneMethod)
		return loc, errors.New("X-Locate-ClientLatLon-Method: " + noneMethod)
	}
	// Second, country is valid, so try to lookup region.
	region := strings.ToUpper(headers.Get("X-AppEngine-Region"))
	if region != "" && static.Regions[country+"-"+region] != "" {
		latlon = static.Regions[country+"-"+region]
		log.WithFields(fields).Info(regionMethod)
		loc, err := splitLatLon(latlon)
		loc.Headers.Set("X-Locate-ClientLatLon", latlon)
		loc.Headers.Set("X-Locate-ClientLatLon-Method", regionMethod)
		return loc, err
	}
	// Third, region was not found, fallback to using the country.
	latlon = static.Countries[country]
	log.WithFields(fields).Info(countryMethod)
	loc, err = splitLatLon(latlon)
	loc.Headers.Set("X-Locate-ClientLatLon", latlon)
	loc.Headers.Set("X-Locate-ClientLatLon-Method", countryMethod)
	return loc, err
}

// Reload does nothing.
func (sl *AppEngineLocator) Reload(ctx context.Context) {}

// splitLatLon attempts to split the "<lat>,<lon>" string provided by AppEngine
// into two fields. The return values preserve the original lat,lon order.
func splitLatLon(latlon string) (*Location, error) {
	loc := &Location{
		// The empty header type is nil, so we set it.
		Headers: http.Header{},
	}
	if latlon == nullLatLon {
		return loc, ErrNullLatLon
	}
	fields := strings.Split(latlon, ",")
	if len(fields) != 2 {
		return loc, ErrBadLatLonFormat
	}
	loc.Latitude = fields[0]
	loc.Longitude = fields[1]
	return loc, nil
}
