package health

import "testing"

func TestChecker_getHealth(t *testing.T) {
	tests := []struct {
		name string
		pp   *PortProbe
		want float64
	}{

		{
			name: "health-1",
			pp:   &PortProbe{},
			want: 1,
		},
		{
			name: "health-0",
			pp: &PortProbe{
				ports: map[string]bool{"65536": true},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewChecker(tt.pp)

			got := hc.GetHealth()
			if got != tt.want {
				t.Errorf("Checker.GetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
