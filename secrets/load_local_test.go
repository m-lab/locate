package secrets_test

import (
	"context"
	"testing"

	"github.com/m-lab/locate/secrets"
)

func TestLocalConfig_LoadSigner(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{
			name: "success",
			file: "testdata/jwk_sig_EdDSA_unittest_20220415",
		},
		{
			name:    "error-badfile",
			file:    "not-testdata/file-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-given-public-key",
			file:    "testdata/jwk_sig_EdDSA_unittest_20220415.pub",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := secrets.NewLocalConfig()
			ctx := context.Background()
			_, err := c.LoadSigner(ctx, nil, tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalConfig.LoadSigner() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestLocalConfig_LoadVerifier(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr bool
	}{
		{
			name: "success",
			file: "testdata/jwk_sig_EdDSA_unittest_20220415.pub",
		},
		{
			name:    "error-badfile",
			file:    "not-testdata/file-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-given-private-key",
			file:    "testdata/jwk_sig_EdDSA_unittest_20220415",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := secrets.NewLocalConfig()
			ctx := context.Background()
			_, err := c.LoadVerifier(ctx, nil, tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalConfig.LoadVerifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
