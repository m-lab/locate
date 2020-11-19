package clientgeo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"
	"github.com/oschwald/geoip2-golang"

	"github.com/m-lab/uuid-annotator/tarreader"
)

// NewMaxmindLocator creates a new MaxmindLocator and loads the current copy of
// MaxMind data stored in GCS.
func NewMaxmindLocator(ctx context.Context, mm content.Provider) *MaxmindLocator {
	mml := &MaxmindLocator{
		dataSource: mm,
	}
	var err error
	mml.maxmind, err = mml.load(ctx)
	rtx.Must(err, "Could not load annotation db")
	return mml
}

// MaxmindLocator is the central struct for this module.
type MaxmindLocator struct {
	mut        sync.RWMutex
	dataSource content.Provider
	maxmind    *geoip2.Reader
}

var emptyResult = geoip2.City{}

// Locate finds the Location of the given IP.
func (mml *MaxmindLocator) Locate(req *http.Request) (*Location, error) {
	mml.mut.RLock()
	defer mml.mut.RUnlock()

	ip, err := ipFromRequest(req)
	if err != nil {
		return nil, err
	}
	if ip == nil {
		return nil, errors.New("cannot locate nil IP")
	}
	if mml.maxmind == nil {
		log.Println("No maxmind DB present. This should only occur during testing.")
		return nil, errors.New("no maxmind db loaded")
	}
	record, err := mml.maxmind.City(ip)
	if err != nil {
		return nil, err
	}

	// Check for empty results because "not found" is not an error. Instead the
	// geoip2 package returns an empty result. May be fixed in a future version:
	// https://github.com/oschwald/geoip2-golang/issues/32
	//
	// "Not found" in a well-functioning database should not be an error.
	// Instead, it is an accurate reflection of data that is missing.
	if isEmpty(record) {
		return nil, errors.New("unknown location; empty result")
	}

	lat := fmt.Sprintf("%f", record.Location.Latitude)
	lon := fmt.Sprintf("%f", record.Location.Longitude)
	tmp := &Location{
		Latitude:  lat,
		Longitude: lon,
		Headers: http.Header{
			"X-Locate-ClientLatLon":        []string{lat + "," + lon},
			"X-Locate-ClientLatLon-Method": []string{"maxmind-remoteip"},
		},
	}
	return tmp, nil
}

func ipFromRequest(req *http.Request) (net.IP, error) {
	fwdIPs := strings.Split(req.Header.Get("X-Forwarded-For"), ", ")
	var ip net.IP
	if fwdIPs[0] != "" {
		ip = net.ParseIP(fwdIPs[0])
	} else {
		h, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			return nil, errors.New("failed to parse remote addr")
		}
		ip = net.ParseIP(h)
	}
	return ip, nil
}

// Reload is intended to be regularly called in a loop. It should check whether
// the data in GCS is newer than the local data, and, if it is, then download
// and load that new data into memory and then replace it in the annotator.
func (mml *MaxmindLocator) Reload(ctx context.Context) {
	mm, err := mml.load(ctx)
	if err != nil {
		log.Println("Could not reload maxmind dataset:", err)
		return
	}
	// Don't acquire the lock until after the data is in RAM.
	mml.mut.Lock()
	defer mml.mut.Unlock()
	mml.maxmind = mm
}

func isEmpty(r *geoip2.City) bool {
	// The record has no associated city, country, or continent.
	return r.City.GeoNameID == 0 && r.Country.GeoNameID == 0 && r.Continent.GeoNameID == 0
}

// load unconditionally loads datasets and returns them.
func (mml *MaxmindLocator) load(ctx context.Context) (*geoip2.Reader, error) {
	tgz, err := mml.dataSource.Get(ctx)
	if err == content.ErrNoChange {
		return mml.maxmind, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := tarreader.FromTarGZ(tgz, "GeoLite2-City.mmdb")
	if err != nil {
		return nil, err
	}
	return geoip2.FromBytes(data)
}
