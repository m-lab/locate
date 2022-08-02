package main

// HealthChecker ...
type HealthChecker struct {
	scanner PortScanner
}

// NewHealthChecker ...
func NewHealthChecker(ps PortScanner) *HealthChecker {
	hc := HealthChecker{
		scanner: ps,
	}
	return &hc
}

func (hc *HealthChecker) getHealth() float64 {
	if hc.scanner.scanPorts() {
		return 1
	}
	return 0
}
