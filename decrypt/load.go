// Package signer loads encrypted keys from Google KMS.
package signer

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
func (c *Config) Load(ctx context.Context, client Decrypter, ciphertext string) (*token.Signer, error) {
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
	return token.NewSigner(resp.Plaintext)
}

// path creates a GCP resource path for the KMS key referenced by Config.
func (c *Config) path() string {
	return "projects/" + c.Project + "/locations/" + c.Region +
		"/keyRings/" + c.Keyring + "/cryptoKeys/" + c.Key
}
