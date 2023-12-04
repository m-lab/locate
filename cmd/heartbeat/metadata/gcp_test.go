package metadata

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestNewGCPMetadata(t *testing.T) {
	tests := []struct {
		name     string
		client   Client
		hostname string
		want     *GCPMetadata
		wantErr  bool
	}{
		{
			name: "success",
			client: &fakeClient{
				proj: "mlab-sandbox",
				zone: "us-west1-a",
			},
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org-t95j",
			want: &GCPMetadata{
				project: "mlab-sandbox",
				backend: "mlab1-lga0t-mlab-sandbox-measurement-lab-org",
				region:  "us-west1",
				group:   fmt.Sprintf(groupTemplate, "mlab-sandbox", "us-west1", "mlab1-lga0t-mlab-sandbox-measurement-lab-org"),
			},
			wantErr: false,
		},
		{
			name: "invalid-hostname",
			client: &fakeClient{
				proj: "mlab-sandbox",
				zone: "us-west1-a",
			},
			hostname: "invalid-hostname",
			want:     nil,
			wantErr:  true,
		},
		{
			name: "invalid-proj",
			client: &fakeClient{
				projErr: true,
			},
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org-t95j",
			want:     nil,
			wantErr:  true,
		},
		{
			name: "invalid-zone",
			client: &fakeClient{
				zoneErr: true,
			},
			hostname: "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org-t95j",
			want:     nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGCPMetadata(tt.client, tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGCPMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewGCPMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

type fakeClient struct {
	proj    string
	projErr bool
	zone    string
	zoneErr bool
}

func (fc *fakeClient) ProjectID() (string, error) {
	if fc.projErr {
		return "", errors.New("project error")
	}
	return fc.proj, nil
}

func (fc *fakeClient) Zone() (string, error) {
	if fc.zoneErr {
		return "", errors.New("zone error")
	}
	return fc.zone, nil
}

func TestGCPMetadata_Project(t *testing.T) {
	wantProj := "fake-project"
	m := &GCPMetadata{
		project: wantProj,
	}
	if got := m.Project(); got != wantProj {
		t.Errorf("GCPMetadata.Project() = %v, want %v", got, wantProj)
	}
}

func TestGCPMetadata_InstanceName(t *testing.T) {
	wantName := "fake-name"
	m := &GCPMetadata{
		backend: wantName,
	}
	if got := m.InstanceName(); got != wantName {
		t.Errorf("GCPMetadata.InstanceName() = %v, want %v", got, wantName)
	}
}

func TestGCPMetadata_Region(t *testing.T) {
	wantRegion := "fake-region"
	m := &GCPMetadata{
		region: wantRegion,
	}
	if got := m.Region(); got != wantRegion {
		t.Errorf("GCPMetadata.Region() = %v, want %v", got, wantRegion)
	}
}

func TestGCPMetadata_Group(t *testing.T) {
	wantGroup := "fake-group"
	m := &GCPMetadata{
		group: wantGroup,
	}
	if got := m.Group(); got != wantGroup {
		t.Errorf("GCPMetadata.Group() = %v, want %v", got, wantGroup)
	}
}
