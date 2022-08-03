package main

// HealthChecker checks the health of a local experiment instance.
type HealthChecker struct {
	pc PortChecker
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(pc PortChecker) *HealthChecker {
	hc := HealthChecker{
		pc: pc,
	}
	return &hc
}

// getHealth combines a set of health checks into a single score.
func (hc *HealthChecker) getHealth() float64 {
	if hc.pc.checkPorts() {
		return 1
	}
	return 0
}
