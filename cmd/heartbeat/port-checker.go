package main

import (
	"log"
	"net"
	"net/url"
	"time"
)

const (
	defaultPort       = "80"
	defaultPortSecure = "443"
)

// PortChecker checks whether a set of ports are open.
type PortChecker struct {
	ports map[string]bool
}

// NewPortChecker creates a new PortChecker.
func NewPortChecker(services map[string][]string) *PortChecker {
	pc := PortChecker{
		ports: getPorts(services),
	}
	return &pc
}

// checkPorts returns true if all the given ports are open and false
// otherwise.
func (ps *PortChecker) checkPorts() bool {
	for p := range ps.ports {
		conn, err := net.DialTimeout("tcp", p, time.Second)
		if err != nil {
			log.Printf("Failed to reach port %s", p)
			return false
		}
		conn.Close()
	}
	return true
}

// getPorts extracts the set of ports from a map of service names to
// their URL templates.
func getPorts(services map[string][]string) map[string]bool {
	ports := make(map[string]bool)

	for _, s := range services {
		for _, u := range s {
			url, err := url.Parse(u)

			if err != nil {
				continue
			}

			port := getPort(*url)
			ports[":"+port] = true
		}
	}

	return ports
}

// getPort extracts the port from a single URL. If no port is specified,
// it sets a default.
func getPort(url url.URL) string {
	port := url.Port()

	// Set default ports.
	if port == "" {
		if url.Scheme == "https" || url.Scheme == "wss" {
			return defaultPortSecure
		}
		return defaultPort
	}

	return port
}
