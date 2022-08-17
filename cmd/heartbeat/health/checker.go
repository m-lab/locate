package health

// Checker checks the health of a local experiment instance.
type Checker struct {
	pp  *PortProbe
	k8s *KubernetesClient
}

// NewChecker creates a new Checker.
func NewChecker(pp *PortProbe, k8s *KubernetesClient) *Checker {
	return &Checker{
		pp:  pp,
		k8s: k8s,
	}
}

// GetHealth combines a set of health checks into a single score.
func (hc *Checker) GetHealth() float64 {
	if hc.pp.checkPorts() && hc.k8s.isHealthy() {
		return 1
	}
	return 0
}
