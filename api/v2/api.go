// Package v2 defines the request API for the location service.
//
// While well provisioned, the M-Lab Platform is finite. On occasion, due to
// peak usage, local service outages, or abnormal client behavior the location
// service must decline to schedule new user requests. This is necesssary to
// safegaurd measurement quality of your measurements and those of others. The
// v2 API classifies user requests into three priorities.
//
//	API-key | Access Token | Priority
//	--------------------------------------------------------
//	YES     | YES          | API-Key, High Availability Pool
//	YES     | NO           | API-Key, Best Effort Pool
//	NO      | NO           | Global Best Effort Pool
//
// For highest priority access to the platform, register an API key and use the
// NearestResult.NextRequest.URL when provided.
package v2

import "time"

// NearestResult is returned by the location service in response to query
// requests.
type NearestResult struct {
	// Error contains information about request failures.
	Error *Error `json:"error,omitempty"`

	// NextRequest defines the earliest time that a client should make a new
	// request using the included URL.
	//
	// Under normal circumstances, NextRequest is provided *with* Results. The
	// next request time is sampled from an exponential distribution such that
	// inter-request times are memoryless. Under abnormal circumstances, such as
	// high single-client request rates or target capacity exhaustion, the next
	// request is provided *without* Results.
	//
	// Non-interactive or batch clients SHOULD schedule measurements with this
	// value. All clients SHOULD NOT make additional requests more often than
	// NextRequest. The server MAY reject requests indefinitely when clients do
	// not respect this limit.
	NextRequest *NextRequest `json:"next_request,omitempty"`

	// Results contains an array of Targets matching the client request.
	Results []Target `json:"results,omitempty"`
}

// MonitoringResult contains one Target with a single-purpose access-token
// useful only for monitoring services on the target machine.
type MonitoringResult struct {
	// Error contains information about request failures.
	Error *Error `json:"error,omitempty"`

	// AccessToken is the access token used in Target URLs. Some applications
	// may use this value instead of specific Target.URLs.
	AccessToken string `json:"access_token"`

	// Results is array of Targets matching the client request. In the case of
	// monitoring requests this array will only contain a single element, but we
	// leave it as an array so that the response of monitoring requests matches
	// the response of normal locate requests so that clients can treat data
	// from either request the same.
	Results []Target `json:"results,omitempty"`
}

// NextRequest contains a URL for scheduling the next request. The URL embeds an
// access token that will be valid after `NotBefore`. The access token will
// remain valid until it `Expires`. If a client uses an expired URL, the request
// will be handled as if no access token were provided, i.e. using a lower
// priority class.
type NextRequest struct {
	// NotBefore defines the time after which the URL will become valid. This
	// value is the same time used in "nbf" field of the underlying JSON Web
	// Token (JWT) claim. To show this equivalence, we use the same name.
	NotBefore time.Time `json:"nbf"`

	// Expires defines the time after which the URL will be invalid. Expires will
	// always be greater than NotBefore. This value is the same time used in the
	// "exp" field of the underlying JWT claim.
	Expires time.Time `json:"exp"`

	// URL should be used to make the next request to the location service.
	URL string `json:"url"`
}

// Location contains metadata about the geographic location of a target machine.
type Location struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

// Target contains information needed to run a measurement to a measurement
// service on a single M-Lab machine. Measurement services may support multiple
// resources. A Target contains at least one measurement service resource in
// URLs.
type Target struct {
	// Machine is the FQDN of the machine hosting the measurement service.
	Machine string `json:"machine"`

	// Hostname is the FQDN of the measurement service targeted in URLs.
	Hostname string `json:"hostname"`

	// Location contains metadata about the geographic location of the target machine.
	Location *Location `json:"location,omitempty"`

	// URLs contains measurement service resource names and the complete URL for
	// running a measurement.
	//
	// A measurement service may support multiple resources (e.g. upload,
	// download, etc). Each key is a resource name and the value is a complete
	// URL with protocol, service name, port, and parameters fully specified.
	URLs map[string]string `json:"urls"`
}

// Error describes an error condition that prevents the server from completing a
// NearestResult.
type Error struct {
	// RFC7807 Fields for "Problem Details".
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// NewError creates a new api Error for a NearestResult.
func NewError(typ, title string, status int) *Error {
	return &Error{
		Type:   typ,
		Title:  title,
		Status: status,
	}
}

// HeartbeatMessage contains pointers to structs of the types
// of messages accepted by the heartbeat service.
type HeartbeatMessage struct {
	Health       *Health
	Registration *Registration
	Prometheus   *Prometheus
}

// Registration contains a set of identifying fields
// for a server instance.
type Registration struct {
	City          string              // City (e.g., New York).
	CountryCode   string              // Country code (e.g., US).
	ContinentCode string              // Continent code (e.g., NA).
	Experiment    string              // Experiment (e.g., ndt).
	Hostname      string              // Fully qualified service hostname.
	Latitude      float64             // Latitude.
	Longitude     float64             // Longitude.
	Machine       string              // Machine (e.g., mlab1).
	Metro         string              // Metro (e.g., lga).
	Project       string              // Project (e.g., mlab-sandbox).
	Probability   float64             // Probability of picking site (e.g., 0.3).
	Site          string              // Site (e.g.. lga01).
	Type          string              // Machine type (e.g., physical, virtual).
	Uplink        string              // Uplink capacity.
	Services      map[string][]string // Mapping of service names.
}

// Health is the structure used by the heartbeat service
// to report health updates.
type Health struct {
	Score float64 // Health score.
}

// Prometheus contains the health data reported by Prometheus.
type Prometheus struct {
	Health bool // Health (e.g., true = healthy).
}
