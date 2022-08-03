package main

import "testing"

func TestHealthChecker_getHealth(t *testing.T) {
	tests := []struct {
		name string
		pc   PortChecker
		want float64
	}{

		{
			name: "health-1",
			pc:   PortChecker{},
			want: 1,
		},
		{
			name: "health-0",
			pc: PortChecker{
				ports: map[string]bool{"65536": true},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker(tt.pc)

			got := hc.getHealth()
			if got != tt.want {
				t.Errorf("HealthChecker.getHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
