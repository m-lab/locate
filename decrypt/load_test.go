// Package decrypt loads encrypted keys from Google KMS.
package decrypt

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/googleapis/gax-go"
	"github.com/m-lab/access/token"
	"github.com/m-lab/go/rtx"
)

type fakeDecrypter struct {
	text  string
	err   error
	given *kmspb.DecryptRequest
}

func (f *fakeDecrypter) Decrypt(ctx context.Context, req *kmspb.DecryptRequest, opts ...gax.CallOption) (*kmspb.DecryptResponse, error) {
	var resp *kmspb.DecryptResponse
	if f.err == nil {
		resp = &kmspb.DecryptResponse{
			Plaintext: []byte(f.text),
		}
	}
	f.given = req
	return resp, f.err
}

var (
	insecurePrivateKey = `{"use":"sig","kty":"OKP","kid":"insecure","crv":"Ed25519","alg":"EdDSA","x":"E50_cwU7ACoH_XM6We3AFLHVWA63xm2crFhKL-PUc3Y","d":"3JRzWpk6aILrhOnry41Fu3u9l0XbloAVhuVNowWqT_Y"}`
	insecurePublicKey  = `{"use":"sig","kty":"OKP","kid":"insecure","crv":"Ed25519","alg":"EdDSA","x":"E50_cwU7ACoH_XM6We3AFLHVWA63xm2crFhKL-PUc3Y"}`
)

func TestConfig_LoadSigner(t *testing.T) {
	tests := []struct {
		name       string
		project    string
		region     string
		keyring    string
		key        string
		text       string
		ciphertext string
		err        error
		wantErr    bool
	}{
		{
			name:       "success",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "signer",
			key:        "private-jwk",
			text:       insecurePrivateKey,
			ciphertext: base64.StdEncoding.EncodeToString([]byte(insecurePrivateKey)),
		},
		{
			name:       "error-decoding",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "signer",
			key:        "private-jwk",
			ciphertext: "%%%%%%this-is-an-invalid-base64-string{}/:%%[]00",
			wantErr:    true,
		},
		{
			name:       "error-decrypting",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "signer",
			key:        "private-jwk",
			ciphertext: "abcdefghijklmnopqrstuvwxyz0123456789",
			err:        errors.New("failure to decrypt"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConfig(tt.project, tt.region, tt.keyring, tt.key)
			ctx := context.Background()
			client := &fakeDecrypter{
				text: tt.text,
				err:  tt.err,
			}
			signer, err := c.LoadSigner(ctx, client, tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if signer == nil {
				return
			}

			// Use the signer to generate and verify a claim.
			cl := jwt.Claims{
				Subject:  "subject",
				Audience: jwt.Audience{"audience"},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
			}
			tok, err := signer.Sign(cl)
			rtx.Must(err, "failed to sign claim")
			v, err := token.NewVerifier([]byte(insecurePublicKey))
			rtx.Must(err, "failed to create verifier")
			exp := jwt.Expected{
				Subject:  "subject",
				Audience: jwt.Audience{"audience"},
				Time:     time.Now(),
			}
			cl2, err := v.Verify(tok, exp)
			rtx.Must(err, "failed to verify")
			if cl.Subject != cl2.Subject || !cl2.Audience.Contains(cl.Audience[0]) {
				t.Errorf("Config.Load() = %v, want %v", cl2, cl)
			}
		})
	}
}

func TestConfig_LoadVerifier(t *testing.T) {
	tests := []struct {
		name       string
		project    string
		region     string
		keyring    string
		key        string
		text       string
		ciphertext string
		err        error
		wantErr    bool
	}{
		{
			name:       "success",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "verifier",
			key:        "private-jwk",
			text:       insecurePublicKey,
			ciphertext: base64.StdEncoding.EncodeToString([]byte(insecurePublicKey)),
		},
		{
			name:       "error-decoding",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "verifier",
			key:        "private-jwk",
			ciphertext: "%%%%%%this-is-an-invalid-base64-string{}/:%%[]00",
			wantErr:    true,
		},
		{
			name:       "error-decrypting",
			project:    "mlab-testing",
			region:     "global",
			keyring:    "verifier",
			key:        "private-jwk",
			ciphertext: "abcdefghijklmnopqrstuvwxyz0123456789",
			err:        errors.New("failure to decrypt"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewConfig(tt.project, tt.region, tt.keyring, tt.key)
			ctx := context.Background()
			client := &fakeDecrypter{
				text: tt.text,
				err:  tt.err,
			}
			verifier, err := c.LoadVerifier(ctx, client, tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if verifier == nil {
				return
			}

			// Use the signer to generate and verify a claim.
			cl := jwt.Claims{
				Subject:  "subject",
				Audience: jwt.Audience{"audience"},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
			}
			signer, err := token.NewSigner([]byte(insecurePrivateKey))
			rtx.Must(err, "failed to sign claim")
			tok, err := signer.Sign(cl)
			rtx.Must(err, "failed to sign claim")
			exp := jwt.Expected{
				Subject:  "subject",
				Audience: jwt.Audience{"audience"},
				Time:     time.Now(),
			}
			cl2, err := verifier.Verify(tok, exp)
			rtx.Must(err, "failed to sign claim")
			if cl.Subject != cl2.Subject || !cl2.Audience.Contains(cl.Audience[0]) {
				t.Errorf("Config.Load() = %v, want %v", cl2, cl)
			}
		})
	}
}
