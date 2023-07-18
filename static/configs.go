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
	RegistrationLoadMin        = 3 * time.Hour
	RegistrationLoadExpected   = 12 * time.Hour
	RegistrationLoadMax        = 24 * time.Hour
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
