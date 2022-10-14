package secrets

import (
	"context"
	"io/ioutil"

	"github.com/m-lab/access/token"
	"github.com/m-lab/locate/prometheus"
	"github.com/prometheus/common/config"
)

// LocalConfig supports loading signer and verifier keys from a local file
// rather than from secretmanager.
type LocalConfig struct{}

// NewLocalConfig creates a new instance for loading local signer and verifier keys.
func NewLocalConfig() *LocalConfig {
	return &LocalConfig{}
}

// LoadSigner reads the secret from the named file. The client parameter is ignored.
func (c *LocalConfig) LoadSigner(ctx context.Context, client SecretClient, name string) (*token.Signer, error) {
	key, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return token.NewSigner(key)
}

// LoadVerifier reads the secret from the named file. The client parameter is ignored.
func (c *LocalConfig) LoadVerifier(ctx context.Context, client SecretClient, name string) (*token.Verifier, error) {
	// TODO: consider supporting `name` as glob to load multiple verifier keys.
	key, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return token.NewVerifier(key)
}

// LoadPrometheus reads the username and password secrets from the named files.
// The client parameter is ignored.
func (c *LocalConfig) LoadPrometheus(ctx context.Context, client SecretClient, user, pass string) (*prometheus.Credentials, error) {
	u, err := ioutil.ReadFile(user)
	if err != nil {
		return nil, err
	}

	p, err := ioutil.ReadFile(pass)
	if err != nil {
		return nil, err
	}

	return &prometheus.Credentials{
		Username: string(u),
		Password: config.Secret(p),
	}, nil
}
