package secrets

import (
	"context"
	"fmt"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go"
	"github.com/m-lab/locate/prometheus"
	"github.com/prometheus/common/config"
	"google.golang.org/api/iterator"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type fakeSecretClient struct {
	idx     int
	data    [][]byte
	wantErr bool
}

func (f *fakeSecretClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if f.wantErr {
		return nil, fmt.Errorf("fake-error")
	}

	defer f.incrementIdx()

	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: "fake-secret",
		Payload: &secretmanagerpb.SecretPayload{
			Data: f.data[f.idx],
		},
	}, nil
}

func (f *fakeSecretClient) ListSecretVersions(ctx context.Context, req *secretmanagerpb.ListSecretVersionsRequest, opts ...gax.CallOption) *secretmanager.SecretVersionIterator {
	return &secretmanager.SecretVersionIterator{}
}

func (f *fakeSecretClient) incrementIdx() {
	f.idx = f.idx + 1
}

type fakeIter struct {
	idx      int
	versions []*secretmanagerpb.SecretVersion
	wantErr  bool
}

func (f *fakeIter) Next(it *secretmanager.SecretVersionIterator) (*secretmanagerpb.SecretVersion, error) {
	if f.wantErr {
		return nil, fmt.Errorf("fake-error")
	}

	defer f.incrementIdx()

	if f.idx == len(f.versions) {
		return nil, iterator.Done
	}
	return &secretmanagerpb.SecretVersion{
		Name:  f.versions[f.idx].Name,
		State: f.versions[f.idx].State,
	}, nil
}

func (f *fakeIter) incrementIdx() {
	f.idx = f.idx + 1
}

func Test_getSecret(t *testing.T) {
	ctx := context.Background()

	var secretData [][]byte
	secretData = append(secretData, []byte("fake-secret"))

	tests := []struct {
		wantErr bool
	}{
		{
			wantErr: false,
		},
		{
			wantErr: true,
		},
	}

	for _, tt := range tests {
		client := &fakeSecretClient{
			data:    secretData,
			wantErr: tt.wantErr,
		}
		cfg := NewConfig("mlab-sandbox", client)
		cfg.iter = &fakeIter{}

		secret, err := cfg.getSecret(ctx, "fake-path")

		if (err != nil) != tt.wantErr {
			t.Fatalf("Got error: %v, but wantErr is %v", err, tt.wantErr)
			return
		}

		if !tt.wantErr {
			if string(secret) != string(secretData[0]) {
				t.Fatalf("Expected secret value '%s', but got: %s", string(secretData[0]), string(secret))
			}
		}
	}
}

func Test_getSecretVersions(t *testing.T) {
	ctx := context.Background()
	client := &fakeSecretClient{}
	cfg := NewConfig("mlab-sandbox", client)

	tests := []struct {
		name             string
		expectedCount    int
		expectedVersions []string
		versions         []*secretmanagerpb.SecretVersion
		wantErr          bool
		wantIterErr      bool
	}{
		{
			name:          "success",
			expectedCount: 2,
			expectedVersions: []string{
				"secrets/mlab-sandbox/fake-secret/versions/3",
				"secrets/mlab-sandbox/fake-secret/versions/1",
			},
			versions: []*secretmanagerpb.SecretVersion{
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/4",
					State: secretmanagerpb.SecretVersion_DISABLED,
				},
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/3",
					State: secretmanagerpb.SecretVersion_ENABLED,
				},
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
					State: secretmanagerpb.SecretVersion_DESTROYED,
				},
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/1",
					State: secretmanagerpb.SecretVersion_ENABLED,
				},
			},
		},
		{
			name: "no-versions-error",
			versions: []*secretmanagerpb.SecretVersion{
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/4",
					State: secretmanagerpb.SecretVersion_DISABLED,
				},
			},
			wantErr:     true,
			wantIterErr: false,
		},
		{
			name:        "iterator-error",
			wantErr:     true,
			wantIterErr: true,
		},
	}

	for _, tt := range tests {
		cfg.iter = &fakeIter{
			wantErr:  tt.wantIterErr,
			versions: tt.versions,
		}
		versions, err := cfg.getSecretVersions(ctx, "test")

		if (err != nil) != tt.wantErr {
			t.Fatalf("Got error: %v, but wantErr is %v", err, tt.wantErr)
			return
		}

		if len(versions) != tt.expectedCount {
			t.Fatalf("Expected %d secret versions, but got %d", tt.expectedCount, len(versions))
		}

		for i, v := range tt.expectedVersions {
			if v != versions[i] {
				t.Fatalf("Expected versions:\n\n%v\n\n...but got:\n\n%v", tt.expectedVersions, versions)
			}
		}
	}
}

func Test_LoadSigner(t *testing.T) {
	ctx := context.Background()

	var signerKeys [][]byte

	signerData := `
		{
			"use": "sig",
			"kty": "OKP",
			"kid": "fake_20210721",
			"crv": "Ed25519",
			"alg": "EdDSA",
			"x": "abcde-abcd-abcdefghijklmnopqrstuv-abcdefghi",
			"d": "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"
		}
	`
	signerKeys = append(signerKeys, []byte(signerData))

	tests := []struct {
		name    string
		client  SecretClient
		iter    iter
		wantErr bool
	}{
		{
			name: "success",
			client: &fakeSecretClient{
				data:    signerKeys,
				wantErr: false,
			},
			iter: &fakeIter{
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/4",
						State: secretmanagerpb.SecretVersion_ENABLED,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get-secret-versions-error",
			client: &fakeSecretClient{
				wantErr: false,
			},
			iter: &fakeIter{
				wantErr: true,
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
						State: secretmanagerpb.SecretVersion_DISABLED,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "get-secret-error",
			client: &fakeSecretClient{
				wantErr: true,
			},
			iter: &fakeIter{
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
						State: secretmanagerpb.SecretVersion_ENABLED,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		cfg := NewConfig("mlab-sandbox", tt.client)
		cfg.iter = tt.iter
		_, err := cfg.LoadSigner(ctx, "test")

		if (err != nil) != tt.wantErr {
			t.Fatalf("Got error: %v, but wantErr is %v", err, tt.wantErr)
		}
	}
}

func Test_LoadVerifier(t *testing.T) {
	ctx := context.Background()

	var verifyKeys [][]byte

	verifyData0 := `
		{
			"use": "sig",
			"kty": "OKP",
			"kid": "fake0_20210721",
			"crv": "Ed25519",
			"alg": "EdDSA",
			"x": "abcde-abcd-abcdefghijklmnopqrstuv-abcdefghi"
		}
	`
	verifyKeys = append(verifyKeys, []byte(verifyData0))

	verifyData1 := `
		{
			"use": "sig",
			"kty": "OKP",
			"kid": "fake1_20210721",
			"crv": "Ed25519",
			"alg": "EdDSA",
			"x": "abcde-abcd-abcdefghijklmnopqrstuv-abcdefghi"
		}
	`
	verifyKeys = append(verifyKeys, []byte(verifyData1))

	tests := []struct {
		name    string
		client  SecretClient
		iter    iter
		wantErr bool
	}{
		{
			name: "success",
			client: &fakeSecretClient{
				data:    verifyKeys,
				wantErr: false,
			},
			iter: &fakeIter{
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/4",
						State: secretmanagerpb.SecretVersion_ENABLED,
					},
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
						State: secretmanagerpb.SecretVersion_ENABLED,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get-secret-versions-error",
			client: &fakeSecretClient{
				wantErr: false,
			},
			iter: &fakeIter{
				wantErr: true,
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
						State: secretmanagerpb.SecretVersion_DISABLED,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "get-secret-error",
			client: &fakeSecretClient{
				wantErr: true,
			},
			iter: &fakeIter{
				versions: []*secretmanagerpb.SecretVersion{
					{
						Name:  "secrets/mlab-sandbox/fake-secret/versions/2",
						State: secretmanagerpb.SecretVersion_ENABLED,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		cfg := NewConfig("mlab-sandbox", tt.client)
		cfg.iter = tt.iter

		_, err := cfg.LoadVerifier(ctx, "test2")

		if (err != nil) != tt.wantErr {
			t.Fatalf("Got error: %v, but wantErr is %v", err, tt.wantErr)
		}
	}
}

func TestConfig_LoadPrometheus(t *testing.T) {
	ctx := context.Background()

	secretUser := "fake-user"
	secretPass := "fake-pass"
	secretData := [][]byte{
		[]byte(secretUser), []byte(secretPass),
	}

	tests := []struct {
		name    string
		client  SecretClient
		want    *prometheus.Credentials
		wantErr bool
	}{
		{
			name: "success",
			client: &fakeSecretClient{
				data: secretData,
			},
			want: &prometheus.Credentials{
				Username: secretUser,
				Password: config.Secret(secretPass),
			},
			wantErr: false,
		},
		{
			name: "get-secret-error",
			client: &fakeSecretClient{
				wantErr: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig("mlab-sandbox", tt.client)

			got, err := cfg.LoadPrometheus(ctx, "fake-user", "fake-pass")

			if (err != nil) != tt.wantErr {
				t.Errorf("Config.LoadPrometheus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Username != tt.want.Username || got.Password != tt.want.Password {
					t.Errorf("Config.LoadPrometheus() got = %v, want = %v", got, tt.want)
				}
			}
		})
	}
}
