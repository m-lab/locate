// Package static contains static information for the locate service.
package static

import (
	"net/url"
)

// Constants used by the locate service, clients, and target servers accepting
// access tokens issued by the locate service.
const (
	IssuerLocate     = "locate"
	IssuerMonitoring = "monitoring"
)

// URL creates inline url.URLs.
func URL(scheme, port, path string) url.URL {
	return url.URL{
		Scheme: scheme,
		Host:   port,
		Path:   path,
	}
}

// Configs is a temporary, static mapping of service names and their set of
// associated ports. Ultimately, this will be discovered dynamically as
// service heartbeats register with the locate service.
var Configs = map[string]Ports{
	"ndt/ndt7": Ports{
		"insecure-upload":   URL("http", "80", "/ndt/v7/upload"),
		"insecure-download": URL("http", "80", "/ndt/v7/download"),
		"upload":            URL("https", "443", "/ndt/v7/upload"),
		"download":          URL("https", "443", "/ndt/v7/download"),
	},
	"ndt/ndt5": Ports{
		// TODO: should we report the raw port? Should we use the envelope
		// service in a focused configuration? Should we retire the raw protocol?
		// TODO: change ws port to 3002.
		"ws":  URL("http", "3001", "/ndt_protocol"),
		"wss": URL("https", "3010", "/ndt_protocol"),
	},
	"wehe/replay": Ports{
		"envelope": URL("https", "443", "/v0/allow"),
	},
}

// Ports maps names to URLs.
type Ports map[string]url.URL

// LegacyServices associates legacy mlab-ns experiment target names with their
// v2 equivalent.
var LegacyServices = map[string]string{
	"ndt/ndt5": "ndt_ssl",
	"ndt/ndt7": "ndt7",
}
