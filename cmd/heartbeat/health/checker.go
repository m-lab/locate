package health

import (
	"golang.org/x/net/context"
)

// Checker checks the health of a local experiment instance.
type Checker struct {
	pp  *PortProbe
	k8s *KubernetesClient
	ec  *EndpointClient
}

// NewChecker creates a new Checker.
func NewChecker(pp *PortProbe, ec *EndpointClient) *Checker {
	return &Checker{
		pp: pp,
		ec: ec,
	}
}

// NewCheckerK8S creates a new Checker for Kubernetes deployments.
func NewCheckerK8S(pp *PortProbe, k8s *KubernetesClient, ec *EndpointClient) *Checker {
	return &Checker{
		pp:  pp,
		k8s: k8s,
		ec:  ec,
	}
}

// GetHealth combines a set of health checks into a single score.
func (c *Checker) GetHealth(ctx context.Context) float64 {
	if !c.pp.checkPorts() {
		return 0
	}

	if c.k8s != nil && !c.k8s.isHealthy(ctx) {
		return 0
	}

	// Some experiments might not support a /health endpoint, so
	// the result is only taken into account if the request error
	// is nil.
	status, err := c.ec.checkHealthEndpoint()
	if err == nil && !status {
		return 0
	}
	return 1
}
