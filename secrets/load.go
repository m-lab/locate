// Package secrets loads secrets from the Google Cloud Secret Manager.
package secrets

import (
	"context"
	"fmt"
	"log"
	"path"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go"
	"github.com/m-lab/access/token"
	"github.com/m-lab/locate/prometheus"
	"github.com/prometheus/common/config"
	"google.golang.org/api/iterator"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// Constants used by the secrets loader.
const (
	latestVersion = "/versions/latest"
)

// SecretClient wraps the AccessSecretVersion function provided by the
// secretmanager.Client.
type SecretClient interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	ListSecretVersions(ctx context.Context, req *secretmanagerpb.ListSecretVersionsRequest, opts ...gax.CallOption) *secretmanager.SecretVersionIterator
}

// iter warps the Next() method of a *secretmanager.SecretVersionIterator.
type iter interface {
	Next(it *secretmanager.SecretVersionIterator) (*secretmanagerpb.SecretVersion, error)
}

// stdIter implements the iter interfaces, and is used to invoke the
// iterator.Next() method.
type stdIter struct{}

// Next invokes the Next() method of a *secretmanager.SecretVersionIterator.
func (s *stdIter) Next(it *secretmanager.SecretVersionIterator) (*secretmanagerpb.SecretVersion, error) {
	return it.Next()
}

// Config contains settings for secrets.
type Config struct {
	iter    iter
	Project string
	client  SecretClient
}

// NewConfig creates a new secret config.
func NewConfig(project string, client SecretClient) *Config {
	return &Config{
		iter:    &stdIter{},
		Project: project,
		client:  client,
	}
}

// getSecret fetches the version of a secret specified by 'path' from the Secret
// Manager API.
func (c *Config) getSecret(ctx context.Context, path string) ([]byte, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: path,
	}

	result, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Payload.Data, nil
}

// getSecretVersions returns a slice of all *enabled* versions for a secret. It
// will ignore disabled for destroyed versions of a secret.
func (c *Config) getSecretVersions(ctx context.Context, name string) ([]string, error) {
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent:   c.path(name),
		PageSize: 1000,
	}

	it := c.client.ListSecretVersions(ctx, req)
	versions := []string{}
	for {
		resp, err := c.iter.Next(it)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if resp.State != secretmanagerpb.SecretVersion_ENABLED {
			continue
		}
		versions = append(versions, resp.Name)
	}

	if len(versions) < 1 {
		return nil, fmt.Errorf("no versions found for secret: %s", name)
	}

	return versions, nil
}

// LoadSigner fetches the oldest enabled version of the named secret containing
// the JWT signer key from the Secret Manager API and returns a *token.Signer.
func (c *Config) LoadSigner(ctx context.Context, name string) (*token.Signer, error) {
	versions, err := c.getSecretVersions(ctx, name)
	if err != nil {
		return nil, err
	}
	log.Printf("Loading JWT private signer key %q", versions[len(versions)-1])
	key, err := c.getSecret(ctx, versions[len(versions)-1])
	if err != nil {
		return nil, err
	}
	return token.NewSigner(key)
}

// LoadVerifier fetches all enabled versions of the named secret containing the
// JWT verifier keys and returns a * token.Verifier.
func (c *Config) LoadVerifier(ctx context.Context, name string) (*token.Verifier, error) {
	versions, err := c.getSecretVersions(ctx, name)
	if err != nil {
		return nil, err
	}
	keys := [][]byte{}
	for _, version := range versions {
		key, err := c.getSecret(ctx, version)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return token.NewVerifier(keys...)
}

// LoadPrometheus fetches the latest version of the named secrets containing the
// Prometheus username and password. It returns a *prometheus.Credentials object.
func (c *Config) LoadPrometheus(ctx context.Context, user, pass string) (*prometheus.Credentials, error) {
	userPath := path.Join(c.path(user), latestVersion)
	u, err := c.getSecret(ctx, userPath)
	if err != nil {
		return nil, err
	}

	passPath := path.Join(c.path(pass), latestVersion)
	p, err := c.getSecret(ctx, passPath)
	if err != nil {
		return nil, err
	}

	return &prometheus.Credentials{
		Username: string(u),
		Password: config.Secret(p),
	}, nil
}

func (c *Config) path(name string) string {
	return "projects/" + c.Project + "/secrets/" + name
}
