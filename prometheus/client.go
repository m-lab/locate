package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
)

// Credentials contains Basic Authentication credentials to
// access Prometheus.
type Credentials struct {
	Username string
	Password config.Secret
}

// NewClient returns a new client for the Prometheus HTTP API.
func NewClient(c *Credentials, addr string) (v1.API, error) {
	promClient, err := api.NewClient(api.Config{
		Address: addr,
		Client: &http.Client{
			Transport: config.NewBasicAuthRoundTripper(c.Username, c.Password, "", &http.Transport{}),
		},
	})
	if err != nil {
		return nil, err
	}

	return v1.NewAPI(promClient), nil
}
