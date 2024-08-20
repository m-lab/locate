package secrets_test

import (
	"context"
	"testing"

	"github.com/m-lab/locate/prometheus"
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
			file: "testdata/jwk_sig_EdDSA_test_20220415",
		},
		{
			name:    "error-badfile",
			file:    "not-testdata/file-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-given-public-key",
			file:    "testdata/jwk_sig_EdDSA_test_20220415.pub",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := secrets.NewLocalConfig()
			ctx := context.Background()
			_, err := c.LoadSigner(ctx, tt.file)
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
			file: "testdata/jwk_sig_EdDSA_test_20220415.pub",
		},
		{
			name:    "error-badfile",
			file:    "not-testdata/file-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-given-private-key",
			file:    "testdata/jwk_sig_EdDSA_test_20220415",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := secrets.NewLocalConfig()
			ctx := context.Background()
			_, err := c.LoadVerifier(ctx, tt.file)
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalConfig.LoadVerifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestLocalConfig_LoadPrometheus(t *testing.T) {
	tests := []struct {
		name     string
		userFile string
		passFile string
		want     *prometheus.Credentials
		wantErr  bool
	}{
		{
			name:     "success",
			userFile: "testdata/prom-auth-user",
			passFile: "testdata/prom-auth-pass",
			want: &prometheus.Credentials{
				Username: "username",
				Password: "password",
			},
			wantErr: false,
		},
		{
			name:     "error-bad-user-file",
			userFile: "file-does-not-exist",
			passFile: "testdata/prom-auth-pass",
			wantErr:  true,
		},
		{
			name:     "error-bad-pass-file",
			userFile: "testdata/prom-auth-user",
			passFile: "file-does-not-exist",
			wantErr:  true,
		},
		{
			name:     "error-bad-user-and-pass-files",
			userFile: "file-does-not-exist",
			passFile: "file-does-not-exist",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := secrets.NewLocalConfig()
			ctx := context.Background()
			got, err := c.LoadPrometheus(ctx, tt.userFile, tt.passFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LocalConfig.LoadPrometheus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Username != tt.want.Username && got.Password != tt.want.Password {
					t.Errorf("LocalConfig.LoadPrometheus() got = %v, want= %v", got, tt.want)
				}
			}
		})
	}
}
