// Package static contains static information for the locate service.
package static

import (
	"net/url"
	"time"
)

// Constants used by the locate service, clients, and target servers accepting
// access tokens issued by the locate service.
const (
	IssuerLocate               = "locate"
	AudienceLocate             = "locate"
	IssuerMonitoring           = "monitoring"
	SubjectMonitoring          = "monitoring"
	WebsocketBufferSize        = 1 << 10 // 1024 bytes.
	WebsocketReadDeadline      = 30 * time.Second
	BackoffInitialInterval     = time.Second
	BackoffRandomizationFactor = 0.5
	BackoffMultiplier          = 2
	BackoffMaxInterval         = 5 * time.Minute
	BackoffMaxElapsedTime      = 0
	HeartbeatPeriod            = 10 * time.Second
	MemorystoreExportPeriod    = 10 * time.Second
	PrometheusCheckPeriod      = time.Minute
	RedisKeyExpirySecs         = 30
	EarthHalfCircumferenceKm   = 20038
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
	"ndt/ndt7": {
		URL("ws", "", "/ndt/v7/upload"),
		URL("ws", "", "/ndt/v7/download"),
		URL("wss", "", "/ndt/v7/upload"),
		URL("wss", "", "/ndt/v7/download"),
	},
	"ndt/ndt5": {
		// TODO: should we report the raw port? Should we use the envelope
		// service in a focused configuration? Should we retire the raw protocol?
		// TODO: change ws port to 3002.
		URL("ws", ":3001", "/ndt_protocol"),
		URL("wss", ":3010", "/ndt_protocol"),
	},
	"neubot/dash": {
		URL("https", "", "/negotiate/dash"),
	},
	"wehe/replay": {
		URL("wss", ":4443", "/v0/envelope/access"),
	},
	"iperf3/test": {
		URL("wss", "", "/v0/envelope/access"),
	},
}

// Ports maps names to URLs.
type Ports []url.URL

// LegacyServices associates legacy mlab-ns experiment target names with their
// v2 equivalent.
var LegacyServices = map[string]string{
	"neubot/dash": "neubot",
	"wehe/replay": "wehe", // TODO: replace with heartbeat health.
	"iperf3/test": "ndt7", // TODO: replace with heartbeat health.
	"ndt/ndt5":    "ndt_ssl",
	"ndt/ndt7":    "ndt7",
}

// SiteProbability defines explicit probabilites for sites that cannot handle
// the current number of requests. The default value is 1.0.
// TODO(github.com/m-lab/locate/issues/92): Make this dynamic.
var SiteProbability = map[string]float64{
	"ams10": 0.3, // virtual site
	"bru06": 0.3, // virtual site
	"cgk01": 0.3, // virtual site
	"chs01": 1.0, // virtual site
	"cmh01": 1.0, // virtual site
	"del03": 0.3, // virtual site
	"dfw09": 1.0, // virtual site
	"fra07": 1.0, // virtual site
	"gru05": 1.0, // virtual site
	"hel01": 1.0, // virtual site
	"hkg04": 0.3, // virtual site
	"iad07": 1.0, // virtual site
	"icn01": 0.3, // virtual site
	"kix01": 0.3, // virtual site
	"las01": 1.0, // virtual site
	"lax07": 1.0, // virtual site
	"lax08": 1.0, // virtual site
	"lhr09": 0.3, // virtual site
	"mad07": 1.0, // virtual site
	"mel01": 0.3, // virtual site
	"mil08": 1.0, // virtual site
	"oma01": 1.0, // virtual site
	"ord07": 0.3, // virtual site
	"par08": 1.0, // virtual site
	"pdx01": 1.0, // virtual site
	"scl05": 1.0, // virtual site
	"sin02": 0.3, // virtual site
	"slc01": 1.0, // virtual site
	"syd07": 0.3, // virtual site
	"tpe02": 0.3, // virtual site
	"waw01": 1.0, // virtual site
	"yul07": 1.0, // virtual site
	"yyz07": 1.0, // virtual site
	"zrh01": 1.0, // virtual site

	"lga1t": 0.5, // 1g site
	"bom01": 0.1, // 1g site
	"lis01": 0.5, // 1g site
	"lju01": 0.5, // 1g site
	"tun01": 0.5, // 1g site
}
