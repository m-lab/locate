// Package secrets loads secrets from the Google Cloud Secret Manager
package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go"
	"github.com/m-lab/access/token"
	"google.golang.org/api/iterator"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// SecretManager wraps the AccessSecretVersion function provided by the secretmanager.Client
type SecretManager interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	ListSecretVersions(ctx context.Context, req *secretmanagerpb.ListSecretVersionsRequest, opts ...gax.CallOption) *secretmanager.SecretVersionIterator
}

// Config contains settings for secrets.
type Config struct {
	Name    string
	Project string
}

// NewConfig creates a new secret config.
func NewConfig(project string) *Config {
	return &Config{
		Project: project,
	}
}

// getSecret fetches the version of a secret specified by 'path' from the Secret
// Manager API.
func (c *Config) getSecret(ctx context.Context, client SecretManager, path string) ([]byte, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: path,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return []byte{}, err
	}

	return result.Payload.Data, nil
}

// LoadSigner fetches the latest version of the named secret containing the JWT
// signer key from the Secret Manager API and returns a *token.Signer.
func (c *Config) LoadSigner(ctx context.Context, client SecretManager, name string) (*token.Signer, error) {
	c.Name = name
	path := c.path("/versions/latest")
	key, err := c.getSecret(ctx, client, path)
	if err != nil {
		return nil, err
	}
	return token.NewSigner(key)
}

// LoadVerifier fetches all enabled versions of the named secret containing the
// JWT verifier keys and returns a * token.Verifier.
func (c *Config) LoadVerifier(ctx context.Context, client SecretManager, name string) (*token.Verifier, error) {
	c.Name = name

	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent:   c.path(""),
		PageSize: 1000,
	}

	it := client.ListSecretVersions(ctx, req)
	keys := [][]byte{}
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		key, err := c.getSecret(ctx, client, resp.Name)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	if len(keys) < 1 {
		return nil, fmt.Errorf("No versions found for secret: %s", name)
	}

	return token.NewVerifier(keys...)
}

// path creates a GCP resource path for the Secret referenced by Config.
func (c *Config) path(version string) string {
	return "projects/" + c.Project + "/secrets/" + c.Name + version
}
