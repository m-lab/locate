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

	"github.com/m-lab/go/flagx"
	v2 "github.com/m-lab/locate/api/v2"
)

// ErrNoAvailableServers is returned when there are no available servers. Batch
// clients should pause before scheduling a new request.
var ErrNoAvailableServers = errors.New("No available M-Lab servers")

// ErrNoUserAgent is a foo.
var ErrNoUserAgent = errors.New("client has no user-agent specified")

// Client is an locate client.
type Client struct {
	// HTTPClient performs all requests. Initialized to http.DefaultClient by
	// NewClient. You may override it for alternate settings.
	HTTPClient *http.Client

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
		// TODO: Distinguish these:
		// * Cloud Endpoint errors have a different JSON structure.
		// * AppEngine 500 gateway failures have no JSON structure.
		return nil, err
	}
	if status != http.StatusOK && reply.Error != nil {
		// TODO: create a derived error using %w.
		return nil, errors.New(reply.Error.Title + ": " + reply.Error.Detail)
	}
	if reply.Results == nil {
		// No explicit error and no results.
		return nil, ErrNoAvailableServers
	}
	return reply.Results, nil
}

// get is an internal function used to perform the request.
func (c *Client) get(ctx context.Context, URL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
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
