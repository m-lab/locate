package health

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestChecker_getHealth(t *testing.T) {
	tests := []struct {
		name string
		pp   *PortProbe
		k8s  *KubernetesClient
		want float64
	}{

		{
			name: "health-1",
			pp:   &PortProbe{},
			k8s: &KubernetesClient{
				clientset: healthyClientset,
			},
			want: 1,
		},
		{
			name: "ports-unhealthy",
			pp: &PortProbe{
				ports: map[string]bool{"65536": true},
			},
			k8s: &KubernetesClient{
				clientset: healthyClientset,
			},
			want: 0,
		},
		{
			name: "kubernetes-unhealthy",
			pp:   &PortProbe{},
			k8s: &KubernetesClient{
				clientset: fake.NewSimpleClientset(),
			},
			want: 0,
		},
		{
			name: "all-unhealthy",
			pp: &PortProbe{
				ports: map[string]bool{"65536": true},
			},
			k8s: &KubernetesClient{
				clientset: fake.NewSimpleClientset(),
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewChecker(tt.pp, tt.k8s)

			got := hc.GetHealth()
			if got != tt.want {
				t.Errorf("Checker.GetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
