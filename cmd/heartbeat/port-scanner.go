package main

import (
	"log"
	"net"
	"net/url"
	"time"
)

// PortScanner ...
type PortScanner struct {
	ports map[string]bool
}

// NewPortScanner ...
func NewPortScanner(services map[string][]string) *PortScanner {
	ps := PortScanner{
		ports: make(map[string]bool),
	}

	for _, s := range services {
		for _, u := range s {
			url, err := url.Parse(u)

			if err != nil {
				continue
			}

			port := url.Port()
			// Set default ports.
			if (url.Scheme == "http" || url.Scheme == "ws") && port == "" {
				port = "80"
			}
			if (url.Scheme == "https" || url.Scheme == "wss") && port == "" {
				port = "443"
			}

			ps.ports[":"+port] = true
		}
	}

	return &ps
}

func (ps *PortScanner) scanPorts() bool {
	for p := range ps.ports {
		conn, err := net.DialTimeout("tcp", p, time.Second)
		if err != nil {
			log.Printf("Failed to reach port %s", p)
			return false
		}
		log.Printf("Successfully reached port %s", p)
		conn.Close()
	}

	return true
}
