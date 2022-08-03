package health

// Checker checks the health of a local experiment instance.
type Checker struct {
	pp *PortProbe
}

// NewChecker creates a new Checker.
func NewChecker(pp *PortProbe) *Checker {
	return &Checker{
		pp: pp,
	}
}

// GetHealth combines a set of health checks into a single score.
func (hc *Checker) GetHealth() float64 {
	if hc.pp.checkPorts() {
		return 1
	}
	return 0
}
