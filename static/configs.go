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
	// TRIALS(soltesz): Until 2023-05-24
	// ATL MIA -> South Carolina chs
	"chs01": 1.0, // virtual site
	"atl02": 0.3,
	"atl03": 0.3,
	"atl04": 0.3,
	"atl07": 0.3,
	"atl08": 0.3,
	"mia02": 0.3,
	"mia03": 0.3,
	"mia04": 0.3,
	"mia05": 0.3,
	"mia06": 0.3,

	// NUQ LAX -> Las Vegas las & Los Angeles lax
	"las01": 1.0, // virtual site
	"lax07": 1.0, // virtual site
	"nuq02": 0.3,
	"nuq03": 0.3,
	"nuq04": 0.3,
	"nuq06": 0.3,
	"nuq07": 0.3,
	"lax02": 0.3,
	"lax03": 0.3,
	"lax04": 0.3,
	"lax05": 0.3,
	"lax06": 0.3,

	// ARN Sweden -> Finland hel01
	"hel01": 1.0, // virtual site
	"arn02": 0.3,
	"arn03": 0.3,
	"arn04": 0.3,
	"arn05": 0.3,
	"arn06": 0.3,
	// SVG (Norway)
	"svg01": 0.3,

	// YVR & SEA -> Oregon pdx01
	"pdx01": 1.0, // virtual site
	"yvr02": 0.3,
	"yvr03": 0.3,
	"yvr04": 0.3,
	"sea02": 0.2, // YVR must get past SEA to reach PDX.
	"sea03": 0.2,
	"sea04": 0.2,
	"sea07": 0.2,
	"sea08": 0.2,

	// DEN -> Salt Lake City slc & oma
	"oma01": 1.0, // virtual site
	"slc01": 1.0, // virtual site
	"den02": 0.3,
	"den04": 0.3,
	"den05": 0.3,
	"den06": 0.3,

	// EZE (Argentina) -> Santiago (Chile) scl
	"scl05": 1.0, // virtual site
	"eze01": 0.3,
	"eze02": 0.3,
	"eze03": 0.3,
	"eze04": 0.3,

	// MRS (France) -> Milan Italy
	"zrh01": 1.0, // virtual site
	"mad07": 1.0, // virtual site
	"mil08": 1.0, // virtual site
	"mrs01": 0.3,
	"mrs02": 0.3,
	"mrs03": 0.3,
	"mrs04": 0.3,
	"mil02": 0.2, // There are many mil sites.
	"mil03": 0.2,
	"mil04": 0.2,
	"mil05": 0.2,
	"mil06": 0.2,
	"mil07": 0.2,
	"trn02": 0.2, // Singleton: Prefer mil08
	"bcn01": 0.2, // Singleton: Prefer nearby metros.

	// TRIAL(github.com/m-lab/ops-tracker/issues/1720) for PRG metro.
	"waw01": 1.0, // virtual site
	"par08": 1.0, // virtual site
	"prg02": 0.3,
	"prg03": 0.3,
	"prg04": 0.3,
	"prg05": 0.3,
	"prg06": 0.3,

	// GIG (Rio) -> GRU (Sao Paulo)
	"gru01": 0.2,
	"gru02": 0.2,
	"gru03": 0.2,
	"gru04": 0.2,
	"gru05": 1.0, // virtual site
	"gig01": 0.2,
	"gig02": 0.2,
	"gig03": 0.2,
	"gig04": 0.2,

	"ams10": 0.3, // virtual site
	"bom01": 0.1, // 1g site
	"bru06": 0.3, // virtual site
	"cgk01": 0.3, // virtual site
	"cmh01": 1.0, // virtual site
	"del03": 0.3, // virtual site
	"dfw09": 0.3, // virtual site
	"fra07": 1.0, // virtual site
	"hkg04": 0.3, // virtual site
	"iad07": 1.0, // virtual site
	"icn01": 0.3, // virtual site
	"kix01": 0.3, // virtual site
	"lax08": 1.0, // virtual site
	"lga1t": 0.5,

	"lhr09": 0.3, // virtual site
	"lis01": 0.5,
	"lju01": 0.5,
	"mel01": 0.3, // virtual site
	"ord07": 0.3, // virtual site

	"sin02": 0.3, // virtual site
	"syd07": 0.3, // virtual site
	"tpe02": 0.3, // virtual site
	"tun01": 0.5,
	"yul07": 1.0, // virtual site
	"yyz07": 1.0, // virtual site
}
