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
	BackoffMaxInterval         = time.Hour
	BackoffMaxElapsedTime      = 0
	MaxReconnectionsTotal      = 10
	MaxReconnectionsTime       = time.Hour
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
	"ams10": 0.1, // virtual site
	"bom01": 0.5,
	"bom02": 0.5,
	"bru06": 0.1,  // virtual site
	"cgk01": 0.1,  // virtual site
	"chs01": 0.1,  // virtual site
	"cmh01": 0.1,  // virtual site
	"del03": 0.1,  // virtual site
	"dfw09": 0.1,  // virtual site
	"fra07": 0.1,  // virtual site
	"gru05": 0.1,  // virtual site
	"hel01": 0.1,  // virtual site
	"hkg04": 0.1,  // virtual site
	"hnd01": 0.05, // 0.05
	"hnd06": 0.1,  // virtual site
	"iad07": 0.1,  // virtual site
	"icn01": 0.1,  // virtual site
	"kix01": 0.1,  // virtual site
	"las01": 0.1,  // virtual site
	"lax07": 0.1,  // virtual site
	"lga1t": 0.5,
	"lhr09": 0.1, // virtual site
	"lis01": 0.5,
	"lju01": 0.5,
	"mad07": 0.1, // virtual site
	"mel01": 0.1, // virtual site
	"mil08": 0.1, // virtual site
	"oma01": 0.1, // virtual site
	"ord07": 0.1, // virtual site
	"par08": 0.1, // virtual site
	"pdx01": 0.1, // virtual site
	"scl05": 0.1, // virtual site
	"sea09": 0.1, // virtual site
	"sin02": 0.1, // virtual site
	"slc01": 0.1, // virtual site
	"syd07": 0.1, // virtual site
	"tpe02": 0.1, // virtual site
	"tun01": 0.5,
	"vie01": 0.5,
	"waw01": 0.1, // virtual site
	"yqm01": 0.5,
	"yul02": 0.2, // 0.2
	"yul07": 0.1, // virtual site
	"yvr01": 0.1, // 0.1
	"ywg01": 0.5,
	"yyc02": 0.5,
	"yyz02": 0.2, // 0.2
	"yyz07": 0.1, // virtual site
	"zrh01": 0.1, // virtual site
}
