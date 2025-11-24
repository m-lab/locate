// Package handler provides a client and handlers for responding to locate
// requests.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/limits"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/siteinfo"
	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var (
	errFailedToLookupClient = errors.New("Failed to look up client location")
	tooManyRequests         = "Too many periodic requests. Please contact support@measurementlab.net."
)

// Signer defines how access tokens are signed.
type Signer interface {
	Sign(cl jwt.Claims) (string, error)
}

type Limiter interface {
	IsLimited(ip, ua string) (limits.LimitStatus, error)
}

// Client contains state needed for xyz.
type Client struct {
	Signer
	project string
	LocatorV2
	ClientLocator
	PrometheusClient
	targetTmpl       *template.Template
	agentLimits      limits.Agents
	ipLimiter        Limiter
	earlyExitClients map[string]bool
	jwtVerifier      Verifier
}

// LocatorV2 defines how the Nearest handler requests machines nearest to the
// client.
type LocatorV2 interface {
	Nearest(service string, lat, lon float64, opts *heartbeat.NearestOptions) (*heartbeat.TargetInfo, error)
	heartbeat.StatusTracker
}

// ClientLocator defines the interface for looking up the client geolocation.
type ClientLocator interface {
	Locate(req *http.Request) (*clientgeo.Location, error)
}

// PrometheusClient defines the interface to query Prometheus.
type PrometheusClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...prom.Option) (model.Value, prom.Warnings, error)
}

type paramOpts struct {
	raw       url.Values
	version   string
	ranks     map[string]int
	svcParams map[string]float64
}

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
}

// NewClient creates a new client.
func NewClient(project string, private Signer, locatorV2 LocatorV2, client ClientLocator,
	prom PrometheusClient, lmts limits.Agents, limiter Limiter, earlyExitClients []string, jwtVerifier Verifier) *Client {
	// Convert slice to map for O(1) lookups
	earlyExitMap := make(map[string]bool)
	for _, client := range earlyExitClients {
		earlyExitMap[client] = true
	}
	return &Client{
		Signer:           private,
		project:          project,
		LocatorV2:        locatorV2,
		ClientLocator:    client,
		PrometheusClient: prom,
		targetTmpl:       template.Must(template.New("name").Parse("{{.Hostname}}{{.Ports}}")),
		agentLimits:      lmts,
		ipLimiter:        limiter,
		earlyExitClients: earlyExitMap,
		jwtVerifier:      jwtVerifier,
	}
}

// NewClientDirect creates a new client with a target template using only the target machine.
func NewClientDirect(project string, private Signer, locatorV2 LocatorV2, client ClientLocator, prom PrometheusClient) *Client {
	return &Client{
		Signer:           private,
		project:          project,
		LocatorV2:        locatorV2,
		ClientLocator:    client,
		PrometheusClient: prom,
		// Useful for the locatetest package when running a local server.
		targetTmpl: template.Must(template.New("name").Parse("{{.Hostname}}{{.Ports}}")),
	}
}

func (c *Client) extraParams(hostname string, index int, p paramOpts) url.Values {
	v := url.Values{}

	// Add client parameters.
	for key := range p.raw {
		if strings.HasPrefix(key, "client_") {
			// note: we only use the first value.
			v.Set(key, p.raw.Get(key))
		}

		val, ok := p.svcParams[key]
		if ok && rand.Float64() < val {
			v.Set(key, p.raw.Get(key))
		}
	}

	// Add early_exit parameter for specified clients
	clientName := p.raw.Get("client_name")
	if clientName != "" && c.earlyExitClients[clientName] {
		v.Set(static.EarlyExitParameter, static.EarlyExitDefaultValue)
	}

	// Add Locate Service version.
	v.Set("locate_version", p.version)

	// Add metro rank.
	rank, ok := p.ranks[hostname]
	if ok {
		v.Set("metro_rank", strconv.Itoa(rank))
	}

	// Add result index.
	v.Set("index", strconv.Itoa(index))

	return v
}

// Nearest uses an implementation of the LocatorV2 interface to look up
// nearest servers.
func (c *Client) Nearest(rw http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	result := v2.NearestResult{}
	setHeaders(rw)

	if c.limitRequest(time.Now().UTC(), req) {
		result.Error = v2.NewError("client", tooManyRequests, http.StatusTooManyRequests)
		writeResult(rw, result.Error.Status, &result)
		metrics.RequestsTotal.WithLabelValues("nearest", "request limit", http.StatusText(result.Error.Status)).Inc()
		return
	}

	// Check rate limit for IP and UA.
	if c.ipLimiter != nil {
		ip := getRemoteAddr(req)
		if ip != "" {
			// An empty UA is technically possible.
			ua := req.Header.Get("User-Agent")
			status, err := c.ipLimiter.IsLimited(ip, ua)
			if err != nil {
				// Log error but don't block request (fail open).
				// TODO: Add tests for this path.
				log.Printf("Rate limiter error: %v", err)
			} else if status.IsLimited {
				// Log IP and UA and block the request.
				result.Error = v2.NewError("client", tooManyRequests, http.StatusTooManyRequests)
				metrics.RequestsTotal.WithLabelValues("nearest", "rate limit",
					http.StatusText(result.Error.Status)).Inc()
				// If the client provided a client_name, we want to know how many times
				// that client_name was rate limited. This may be empty, which is fine.
				clientName := req.Form.Get("client_name")
				metrics.RateLimitedTotal.WithLabelValues(clientName, status.LimitType).Inc()

				log.Printf("Rate limit (%s) exceeded for IP: %s, client: %s, UA: %s", ip,
					status.LimitType, clientName, ua)
				writeResult(rw, result.Error.Status, &result)
				return
			}
		} else {
			// This should never happen if Locate is deployed on AppEngine.
			log.Println("Cannot find IP address for rate limiting.")
		}
	}

	experiment, service := getExperimentAndService(req.URL.Path)

	// Look up client location.
	loc, err := c.checkClientLocation(rw, req)
	if err != nil {
		status := http.StatusServiceUnavailable
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", status)
		writeResult(rw, result.Error.Status, &result)
		metrics.RequestsTotal.WithLabelValues("nearest", "client location",
			http.StatusText(result.Error.Status)).Inc()
		return
	}

	// Parse client location.
	lat, errLat := strconv.ParseFloat(loc.Latitude, 64)
	lon, errLon := strconv.ParseFloat(loc.Longitude, 64)
	if errLat != nil || errLon != nil {
		result.Error = v2.NewError("client", errFailedToLookupClient.Error(), http.StatusInternalServerError)
		writeResult(rw, result.Error.Status, &result)
		metrics.RequestsTotal.WithLabelValues("nearest", "parse client location",
			http.StatusText(result.Error.Status)).Inc()
		return
	}

	// Find the nearest targets using the client parameters.
	q := req.URL.Query()
	t := q.Get("machine-type")
	country := req.Header.Get("X-AppEngine-Country")
	sites := q["site"]
	org := q.Get("org")
	strict := false
	if qsStrict, err := strconv.ParseBool(q.Get("strict")); err == nil {
		strict = qsStrict
	}
	// If strict, override the country from the AppEngine header with the one in
	// the querystring.
	if strict {
		country = q.Get("country")
	}
	opts := &heartbeat.NearestOptions{Type: t, Country: country, Sites: sites, Org: org, Strict: strict}
	targetInfo, err := c.LocatorV2.Nearest(service, lat, lon, opts)
	if err != nil {
		result.Error = v2.NewError("nearest", "Failed to lookup nearest machines", http.StatusInternalServerError)
		writeResult(rw, result.Error.Status, &result)
		metrics.RequestsTotal.WithLabelValues("nearest", "server location",
			http.StatusText(result.Error.Status)).Inc()
		return
	}

	pOpts := paramOpts{
		raw:       req.Form,
		version:   "v2",
		ranks:     targetInfo.Ranks,
		svcParams: static.ServiceParams,
	}
	// Populate target URLs and write out response.
	c.populateURLs(targetInfo.Targets, targetInfo.URLs, experiment, pOpts)
	result.Results = targetInfo.Targets
	writeResult(rw, http.StatusOK, &result)
	metrics.RequestsTotal.WithLabelValues("nearest", "success", http.StatusText(http.StatusOK)).Inc()
}

// Live is a minimal handler to indicate that the server is operating at all.
func (c *Client) Live(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

// Ready reports whether the server is working as expected and ready to serve requests.
func (c *Client) Ready(rw http.ResponseWriter, req *http.Request) {
	if c.LocatorV2.Ready() {
		fmt.Fprintf(rw, "ok")
	} else {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "not ready")
	}
}

// Registrations returns information about registered machines. There are 3
// supported query parameters:
//
// * format - defines the format of the returned JSON
// * org - limits results to only records for the given organization
// * exp - limits results to only records for the given experiment (e.g., ndt)
//
// The "org" and "exp" query parameters are currently only supported by the
// default or "machines" format.
func (c *Client) Registrations(rw http.ResponseWriter, req *http.Request) {
	var err error
	var result interface{}

	setHeaders(rw)

	q := req.URL.Query()
	format := q.Get("format")

	switch format {
	case "geo":
		result, err = siteinfo.Geo(c.LocatorV2.Instances(), q)
	default:
		result, err = siteinfo.Hosts(c.LocatorV2.Instances(), q)
	}

	if err != nil {
		v2Error := v2.NewError("siteinfo", err.Error(), http.StatusInternalServerError)
		writeResult(rw, http.StatusInternalServerError, v2Error)
		return
	}

	writeResult(rw, http.StatusOK, result)
}

// checkClientLocation looks up the client location and copies the location
// headers to the response writer.
func (c *Client) checkClientLocation(rw http.ResponseWriter, req *http.Request) (*clientgeo.Location, error) {
	// Lookup the client location using the client request.
	loc, err := c.Locate(req)
	if err != nil {
		return nil, errFailedToLookupClient
	}

	// Copy location headers to response writer.
	for key := range loc.Headers {
		rw.Header().Set(key, loc.Headers.Get(key))
	}

	return loc, nil
}

// populateURLs populates each set of URLs using the target configuration.
func (c *Client) populateURLs(targets []v2.Target, ports static.Ports, exp string, pOpts paramOpts) {
	for i, target := range targets {
		token := c.getAccessToken(target.Machine, exp)
		params := c.extraParams(target.Machine, i, pOpts)
		targets[i].URLs = c.getURLs(ports, target.Hostname, token, params)
	}
}

// getAccessToken allocates a new access token using the given machine name as
// the intended audience and the subject as the target service.
func (c *Client) getAccessToken(machine, subject string) string {
	// Create the token. The same access token is reused for every URL of a
	// target port.
	// A uuid is added to the claims so that each new token is unique.
	cl := jwt.Claims{
		Issuer:   static.IssuerLocate,
		Subject:  subject,
		Audience: jwt.Audience{machine},
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
		ID:       uuid.NewString(),
	}
	token, err := c.Sign(cl)
	// Sign errors can only happen due to a misconfiguration of the key.
	// A good config will remain good.
	rtx.PanicOnError(err, "signing claims has failed")
	return token
}

// getURLs creates URLs for the named experiment, running on the named machine
// for each given port. Every URL will include an `access_token=` parameter,
// authorizing the measurement.
func (c *Client) getURLs(ports static.Ports, hostname, token string, extra url.Values) map[string]string {
	urls := map[string]string{}
	// For each port config, prepare the target url with access_token and
	// complete host field.
	for _, target := range ports {
		name := target.String()
		params := url.Values{}
		params.Set("access_token", token)
		for key := range extra {
			// note: we only use the first value.
			params.Set(key, extra.Get(key))
		}
		target.RawQuery = params.Encode()

		host := &bytes.Buffer{}
		err := c.targetTmpl.Execute(host, map[string]string{
			"Hostname": hostname,
			"Ports":    target.Host, // from URL template, so typically just the ":port".
		})
		rtx.PanicOnError(err, "bad template evaluation")
		target.Host = host.String()
		urls[name] = target.String()
	}
	return urls
}

// limitRequest determines whether a client request should be rate-limited.
func (c *Client) limitRequest(now time.Time, req *http.Request) bool {
	agent := req.Header.Get("User-Agent")
	l, ok := c.agentLimits[agent]
	if !ok {
		// No limit defined for user agent.
		return false
	}
	return l.IsLimited(now)
}

// setHeaders sets the response headers for "nearest" requests.
func setHeaders(rw http.ResponseWriter) {
	// Set CORS policy to allow third-party websites to use returned resources.
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	// Prevent caching of result.
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
	rw.Header().Set("Cache-Control", "no-store")
}

// writeResult marshals the result and writes the result to the response writer.
func writeResult(rw http.ResponseWriter, status int, result interface{}) {
	b, err := json.MarshalIndent(result, "", "  ")
	// Errors are only possible when marshalling incompatible types, like functions.
	rtx.PanicOnError(err, "Failed to format result")
	rw.WriteHeader(status)
	rw.Write(b)
}

// getExperimentAndService takes an http request path and extracts the last two
// fields. For correct requests (e.g. "/v2/nearest/ndt/ndt5"), this will be the
// experiment name (e.g. "ndt") and the datatype (e.g. "ndt5").
func getExperimentAndService(p string) (string, string) {
	datatype := path.Base(p)
	experiment := path.Base(path.Dir(p))
	return experiment, experiment + "/" + datatype
}

// getRemoteAddr extracts the remote address from the request. When running on
// Google App Engine, the X-Forwarded-For is guaranteed to be set. When running
// elsewhere (including on the local machine), the RemoteAddr from the request
// is used instead.
func getRemoteAddr(req *http.Request) string {
	ip := req.Header.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	} else {
		// Fall back to RemoteAddr for local testing or deployments outside of GAE.
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			// As a last resort, use the whole RemoteAddr.
			ip = strings.TrimSpace(req.RemoteAddr)
		} else {
			ip = host
		}
	}
	return ip
}
