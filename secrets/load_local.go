package secrets

import (
	"context"
	"io/ioutil"

	"github.com/m-lab/access/token"
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
