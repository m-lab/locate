// Package decrypt loads encrypted keys from Google KMS.
package decrypt

import (
	"context"
	"encoding/base64"

	"github.com/googleapis/gax-go"
	"github.com/m-lab/access/token"

	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
)

// Decrypter wraps the Decrypt operation provided by the kms.KeyManagementClient.
type Decrypter interface {
	Decrypt(ctx context.Context, req *kmspb.DecryptRequest, opts ...gax.CallOption) (*kmspb.DecryptResponse, error)
}

// Config contains settings for KMS decryption.
type Config struct {
	Project string
	Region  string
	Keyring string
	Key     string
}

// NewConfig creates a new signer config.
func NewConfig(project, region, keyring, key string) *Config {
	return &Config{
		Project: project,
		Region:  region,
		Keyring: keyring,
		Key:     key,
	}
}

// Load decrypts the ciphertext using the given KMS keypath and creates a new
// token signer from the decrypted text.
func (c *Config) Load(ctx context.Context, client Decrypter, ciphertext string) ([]byte, error) {
	// ciphertext is base64 encoded; before decrypting, decode the ciphertext.
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	// Prepare decrypt request parameters.
	req := &kmspb.DecryptRequest{
		Name:       c.path(),
		Ciphertext: data,
	}

	// Decrypt the data.
	resp, err := client.Decrypt(ctx, req)
	if err != nil {
		return nil, err
	}

	// The plain text should be the original key.
	return resp.Plaintext, nil
}

// LoadSigner decryptes the given ciphertext and uses it to initialize a token Signer.
func (c *Config) LoadSigner(ctx context.Context, client Decrypter, ciphertext string) (*token.Signer, error) {
	b, err := c.Load(ctx, client, ciphertext)
	if err != nil {
		return nil, err
	}
	return token.NewSigner(b)
}

// LoadVerifier decryptes the given ciphertext and uses it to initialize a token Verifier.
func (c *Config) LoadVerifier(ctx context.Context, client Decrypter, ciphertext string) (*token.Verifier, error) {
	b, err := c.Load(ctx, client, ciphertext)
	if err != nil {
		return nil, err
	}
	return token.NewVerifier(b)
}

// path creates a GCP resource path for the KMS key referenced by Config.
func (c *Config) path() string {
	return "projects/" + c.Project + "/locations/" + c.Region +
		"/keyRings/" + c.Keyring + "/cryptoKeys/" + c.Key
}
