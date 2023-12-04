package metadata

import (
	"fmt"
	"strings"

	"github.com/m-lab/go/host"
)

const groupTemplate = "https://www.googleapis.com/compute/v1/projects/%s/regions/%s/instanceGroups/%s"

// GCPMetadata contains metadata about a GCP VM.
type GCPMetadata struct {
	project string
	backend string
	region  string
	group   string
}

// Client uses HTTP requests to query the metadata service.
type Client interface {
	ProjectID() (string, error)
	Zone() (string, error)
}

// NewGCPMetadata returns a new instance of GCPMetadata.
func NewGCPMetadata(c Client, hostname string) (*GCPMetadata, error) {
	h, err := host.Parse(hostname)
	if err != nil {
		return nil, err
	}
	backend := strings.ReplaceAll(h.String(), ".", "-")

	project, err := c.ProjectID()
	if err != nil {
		return nil, err
	}

	zone, err := c.Zone()
	if err != nil {
		return nil, err
	}
	region := zone[:len(zone)-2]

	return &GCPMetadata{
		project: project,
		backend: backend,
		region:  region,
		group:   fmt.Sprintf(groupTemplate, project, region, backend),
	}, nil
}

// Project ID (e.g., mlab-sandbox).
func (m *GCPMetadata) Project() string {
	return m.project
}

// InstanceName in GCE.
func (m *GCPMetadata) InstanceName() string {
	return m.backend
}

// Region derived from zone (e.g., us-west1).
func (m *GCPMetadata) Region() string {
	return m.region
}

// Group is the the URI referencing the instance group.
func (m *GCPMetadata) Group() string {
	return m.group
}
