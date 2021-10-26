// Package secrets loads secrets from the Google Cloud Secret Manager.
package secrets

import (
	"context"
	"fmt"
	"log"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go"
	"github.com/m-lab/access/token"
	"google.golang.org/api/iterator"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
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
	Name    string
	Project string
}

// NewConfig creates a new secret config.
func NewConfig(project string) *Config {
	return &Config{
		iter:    &stdIter{},
		Project: project,
	}
}

// getSecret fetches the version of a secret specified by 'path' from the Secret
// Manager API.
func (c *Config) getSecret(ctx context.Context, client SecretClient, path string) ([]byte, error) {
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: path,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	return result.Payload.Data, nil
}

// getSecretVersions returns a slice of all *enabled* versions for a secret. It
// will ignore disabled for destroyed versions of a secret.
func (c *Config) getSecretVersions(ctx context.Context, client SecretClient) ([]string, error) {
	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent:   c.path(),
		PageSize: 1000,
	}

	it := client.ListSecretVersions(ctx, req)
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
		return nil, fmt.Errorf("no versions found for secret: %s", c.Name)
	}

	return versions, nil
}

// LoadSigner fetches the oldest enabled version of the named secret containing
// the JWT signer key from the Secret Manager API and returns a *token.Signer.
func (c *Config) LoadSigner(ctx context.Context, client SecretClient) (*token.Signer, error) {
	versions, err := c.getSecretVersions(ctx, client)
	if err != nil {
		return nil, err
	}
	log.Printf("Loading JWT private signer key %v", versions[len(versions)-1])
	key, err := c.getSecret(ctx, client, versions[len(versions)-1])
	if err != nil {
		return nil, err
	}
	return token.NewSigner(key)
}

// LoadVerifier fetches all enabled versions of the named secret containing the
// JWT verifier keys and returns a * token.Verifier.
func (c *Config) LoadVerifier(ctx context.Context, client SecretClient) (*token.Verifier, error) {
	versions, err := c.getSecretVersions(ctx, client)
	if err != nil {
		return nil, err
	}
	keys := [][]byte{}
	for _, version := range versions {
		key, err := c.getSecret(ctx, client, version)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return token.NewVerifier(keys...)
}

func (c *Config) path() string {
	return "projects/" + c.Project + "/secrets/" + c.Name
}
