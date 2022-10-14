package prometheus

import "github.com/prometheus/common/config"

// Credentials contains Basic Authentication credentials to
// access Prometheus.
type Credentials struct {
	Username string
	Password config.Secret
}
