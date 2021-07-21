package secrets

import (
	"context"
	"fmt"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go"
	"google.golang.org/api/iterator"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type fakeSecretClient struct {
	data    []byte
	wantErr bool
}

func (f *fakeSecretClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if !f.wantErr {
		return &secretmanagerpb.AccessSecretVersionResponse{
			Name: "fake-secret",
			Payload: &secretmanagerpb.SecretPayload{
				Data: f.data,
			},
		}, nil
	}

	if f.wantErr {
		return nil, fmt.Errorf("fake-error")
	}

	return &secretmanagerpb.AccessSecretVersionResponse{}, nil
}

func (f *fakeSecretClient) ListSecretVersions(ctx context.Context, req *secretmanagerpb.ListSecretVersionsRequest, opts ...gax.CallOption) *secretmanager.SecretVersionIterator {
	return &secretmanager.SecretVersionIterator{}
}

type fakeIter struct {
	idx      int
	versions []*secretmanagerpb.SecretVersion
	err      string
}

func (f *fakeIter) Next(it *secretmanager.SecretVersionIterator) (*secretmanagerpb.SecretVersion, error) {
	defer f.incrementIdx()

	if f.err == "no-errors" {
		if f.idx == len(f.versions) {
			return nil, iterator.Done
		}
		return &secretmanagerpb.SecretVersion{
			Name:  f.versions[f.idx].Name,
			State: f.versions[f.idx].State,
		}, nil
	}

	if f.err == "no-versions" {
		if f.idx == len(f.versions) {
			return nil, iterator.Done
		}
		return &secretmanagerpb.SecretVersion{
			Name:  f.versions[f.idx].Name,
			State: f.versions[f.idx].State,
		}, nil
	}

	if f.err == "iterator-error" {
		return nil, fmt.Errorf("fake-error")
	}

	return nil, iterator.Done
}

func (f *fakeIter) incrementIdx() {
	f.idx = f.idx + 1
}

func Test_getSecret(t *testing.T) {
	ctx := context.Background()

	cfg := NewConfig("mlab-sandbox")
	cfg.iter = &fakeIter{}

	secretData := []byte("fake-secret")
	client := &fakeSecretClient{
		data:    secretData,
		wantErr: false,
	}

	secret, err := cfg.getSecret(ctx, client, "fake-path")
	if err != nil {
		t.Fatalf("Did not expect an error, but got error: %s", err)
	}
	if string(secret) != string(secretData) {
		t.Fatalf("Expected secret value '%s', but got: %s", string(secretData), string(secret))
	}

	client.wantErr = true

	_, err = cfg.getSecret(ctx, client, "fake-path")
	if err == nil {
		t.Fatal("Wanted error, but did not get one.")
	}
}

func Test_getSecretVersions(t *testing.T) {
	ctx := context.Background()
	cfg := NewConfig("mlab-sandbox")
	client := &fakeSecretClient{}

	tests := []struct {
		name             string
		expectedCount    int
		expectedVersions []string
		versions         []*secretmanagerpb.SecretVersion
		wantErr          bool
	}{
		{
			name:          "no-errors",
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
			wantErr: false,
		},
		{
			name: "no-versions",
			versions: []*secretmanagerpb.SecretVersion{
				{
					Name:  "secrets/mlab-sandbox/fake-secret/versions/4",
					State: secretmanagerpb.SecretVersion_DISABLED,
				},
			},
			wantErr: true,
		},
		{
			name:    "iterator-error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		cfg.iter = &fakeIter{
			err:      tt.name,
			versions: tt.versions,
		}
		versions, err := cfg.getSecretVersions(ctx, client)

		if (err != nil) != tt.wantErr {
			t.Fatalf("Did not expect and error, but got error: %s", err)
		}

		if len(versions) != tt.expectedCount {
			t.Fatalf("Expected 2 secret versions, but got %d", len(versions))
		}

		for i, v := range tt.expectedVersions {
			if v != versions[i] {
				t.Fatalf("Expected versions:\n\n%v\n\n...but got:\n\n%v", tt.expectedVersions, versions)
			}
		}
	}
}
