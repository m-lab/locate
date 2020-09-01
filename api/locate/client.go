// Package locate implements a client for the Locate API v2.
package locate

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/m-lab/go/flagx"
	v2 "github.com/m-lab/locate/api/v2"
)

// DefaultTimeout is the default request timeout.
const DefaultTimeout = 15 * time.Second

// ErrNoAvailableServers is returned when there are no available servers. A
// background client should treat this error specially as described in the
// specification of the ndt7 protocol.
var ErrNoAvailableServers = errors.New("No available M-Lab servers")

// ErrQueryFailed indicates a non-200 status code.
var ErrQueryFailed = errors.New("locate API returned 4xx status code")

// ErrRetry indicates an error that should be retried.
var ErrRetry = errors.New("returned non-200 status code")

// ErrServerError is a foo.
var ErrServerError = errors.New("5xx status code")

// ErrNoUserAgent is a foo.
var ErrNoUserAgent = errors.New("client has no user-agent specified")

// Client is an locate client.
type Client struct {
	// HTTPClient is the client that will perform the request. By default
	// it is initialized to http.DefaultClient. You may override it for
	// testing purpses and more generally whenever you are not satisfied
	// with the behaviour of the default HTTP client.
	HTTPClient *http.Client

	// Timeout is the optional maximum amount of time we're willing to wait
	// for mlabns to respond. This setting is initialized by NewClient to its
	// default value, but you may override it.
	Timeout time.Duration

	// UserAgent is the mandatory user agent to be used. Also this
	// field is initialized by NewClient.
	UserAgent string

	// BaseURL is the base url used to contact the Locate API.
	BaseURL *url.URL
}

// baseURL is the default base URL.
var baseURL = flagx.MustNewURL("https://locate.measurementlab.net/v2/nearest/")

func init() {
	flag.Var(&baseURL, "locate.url", "The base url for the Locate API")
}

// NewClient creates a new Client instance. The userAgent must not be empty.
// NewClient sets the BaseURL to the -locate.url flag.
func NewClient(userAgent string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		Timeout:    DefaultTimeout,
		UserAgent:  userAgent,
		BaseURL:    baseURL.URL,
	}
}

// Nearest returns a slice of nearby mlab servers. Returns an error on failure.
func (c *Client) Nearest(ctx context.Context, service string) ([]v2.Target, error) {
	var data []byte
	var err error
	var status int
	reqURL := *c.BaseURL
	reqURL.Path = path.Join(reqURL.Path, service)
	data, status, err = c.get(ctx, reqURL.String())
	if err != nil {
		return nil, err
	}
	reply := &v2.NearestResult{}
	err = json.Unmarshal(data, reply)
	if err != nil {
		// Cloud Endpoint errors have a different JSON structure.
		// AppEngine 500 failures have no structure.
		return nil, err
	}
	if status != http.StatusOK && reply.Error != nil {
		// TODO: create a derived error using %w.
		return nil, errors.New(reply.Error.Title + ": " + reply.Error.Detail)
	}
	if reply.Results == nil {
		// Not an explicit error, and no results.
		return nil, ErrNoAvailableServers
	}
	return reply.Results, nil
}

// get is an internal function used to perform the request.
func (c *Client) get(ctx context.Context, URL string) ([]byte, int, error) {
	reqctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqctx, http.MethodGet, URL, nil)
	if err != nil {
		// e.g. due to an invalid parameter.
		return nil, 0, err
	}
	if c.UserAgent == "" {
		// user agent is required.
		return nil, 0, ErrNoUserAgent
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}
