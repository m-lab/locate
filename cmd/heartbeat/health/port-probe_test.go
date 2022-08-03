package health

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

func TestPortProbe_scanPorts(t *testing.T) {
	tests := []struct {
		name            string
		servers         int
		unreachablePort string
		want            bool
	}{
		{
			name:    "no-ports",
			servers: 0,
			want:    true,
		},
		{
			name:    "one-open-port",
			servers: 1,
			want:    true,
		},
		{
			name:    "multiple-open-ports",
			servers: 3,
			want:    true,
		},
		{
			name:            "one-unreachable-port",
			servers:         0,
			unreachablePort: "65536",
			want:            false,
		},
		{
			name:            "open-and-unreachable-ports",
			servers:         2,
			unreachablePort: "65536",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcs := make(map[string][]string)

			for i := 0; i < tt.servers; i++ {
				mux := http.NewServeMux()
				srv := httptest.NewServer(mux)
				defer srv.Close()

				svcs[strconv.Itoa(i)] = []string{srv.URL}
			}

			if tt.unreachablePort != "" {
				svcs["unreachable"] = []string{tt.unreachablePort}
			}

			pp := NewPortProbe(svcs)

			got := pp.checkPorts()
			if got != tt.want {
				t.Errorf("PortProbe.scanPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPorts(t *testing.T) {
	tests := []struct {
		name     string
		services map[string][]string
		want     map[string]bool
	}{
		{
			name: "invalid-url",
			services: map[string][]string{
				"ndt/ndt7": {
					"url%",
				},
			},
			want: map[string]bool{},
		},
		{
			name: "with-port",
			services: map[string][]string{
				"ndt/ndt5": {
					"ws://:3001/ndt_protocol",
					"wss://:3010/ndt_protocol",
				},
			},
			want: map[string]bool{
				"3001": true,
				"3010": true,
			},
		},
		{
			name: "without-port-ws",
			services: map[string][]string{
				"ndt/ndt7": {
					"ws:///ndt/v7/download",
					"ws:///ndt/v7/upload",
				},
			},
			want: map[string]bool{
				"80": true,
			},
		},
		{
			name: "without-port-wss",
			services: map[string][]string{
				"ndt/ndt7": {
					"wss:///ndt/v7/download",
					"wss:///ndt/v7/upload",
				},
			},
			want: map[string]bool{
				"443": true,
			},
		},
		{
			name: "all-types",
			services: map[string][]string{
				"ndt/ndt7": {
					"ws:///ndt/v7/download",
					"ws:///ndt/v7/upload",
					"wss:///ndt/v7/download",
					"wss:///ndt/v7/upload",
				},
				"ndt/ndt5": {
					"ws://:3001/ndt_protocol",
					"wss://:3010/ndt_protocol",
				},
			},
			want: map[string]bool{
				"80":   true,
				"443":  true,
				"3001": true,
				"3010": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPorts(tt.services)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPorts() = %v, want %v", got, tt.want)
			}
		})
	}
}
