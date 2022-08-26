package health

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestChecker_getHealth(t *testing.T) {
	tests := []struct {
		name    string
		checker *Checker
		want    float64
	}{
		{
			name: "health-1",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: healthyClientset,
				},
			),
			want: 1,
		},
		{
			name: "health-1-k8s-nil",
			checker: NewChecker(
				&PortProbe{},
			),
			want: 1,
		},
		{
			name: "ports-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{
					ports: map[string]bool{"65536": true},
				},
				&KubernetesClient{
					clientset: healthyClientset,
				},
			),
			want: 0,
		},
		{
			name: "kubernetes-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: fake.NewSimpleClientset(),
				},
			),
			want: 0,
		},
		{
			name: "all-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{
					ports: map[string]bool{"65536": true},
				},
				&KubernetesClient{
					clientset: fake.NewSimpleClientset(),
				},
			),
			want: 0,
		},
		{
			name: "all-unhealthy-k8s-nil",
			checker: NewChecker(
				&PortProbe{
					ports: map[string]bool{"65536": true},
				},
			),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.checker.GetHealth(context.Background())
			if got != tt.want {
				t.Errorf("Checker.GetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
