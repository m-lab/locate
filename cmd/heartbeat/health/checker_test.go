package health

import (
	"context"
	"testing"

	"github.com/m-lab/locate/cmd/heartbeat/health/healthtest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestChecker_getHealth(t *testing.T) {
	tests := []struct {
		name           string
		checker        *Checker
		endpointStatus int
		want           float64
	}{
		{
			name: "health-1",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: healthyClientset,
				},
			),
			endpointStatus: 200,
			want:           1,
		},
		{
			name: "health-1-k8s-nil",
			checker: NewChecker(
				&PortProbe{},
			),
			endpointStatus: 200,
			want:           1,
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
			endpointStatus: 200,
			want:           0,
		},
		{
			name: "kubernetes-call-fail",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: fake.NewSimpleClientset(),
				},
			),
			endpointStatus: 200,
			want:           1,
		},
		{
			name: "kubernetes-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: fake.NewSimpleClientset(
						&v1.Pod{
							Status: v1.PodStatus{
								Phase: "Pending",
							},
						},
						&v1.Node{
							Status: v1.NodeStatus{
								Conditions: []v1.NodeCondition{
									{Type: "Ready", Status: "False"},
								},
							},
						},
					),
				},
			),
			endpointStatus: 200,
			want:           0,
		},
		{
			name: "endpoint-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{},
				&KubernetesClient{
					clientset: healthyClientset,
				},
			),
			endpointStatus: 500,
			want:           0,
		},
		{
			name: "all-unhealthy",
			checker: NewCheckerK8S(
				&PortProbe{
					ports: map[string]bool{"65536": true},
				},
				&KubernetesClient{
					clientset: fake.NewSimpleClientset(
						&v1.Pod{
							Status: v1.PodStatus{
								Phase: "Pending",
							},
						},
						&v1.Node{
							Status: v1.NodeStatus{
								Conditions: []v1.NodeCondition{
									{Type: "Ready", Status: "False"},
								},
							},
						},
					),
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
			if tt.endpointStatus != 0 {
				srv := healthtest.TestHealthServer(tt.endpointStatus)
				healthAddress = srv.URL + "/health"
				defer srv.Close()
			}

			got := tt.checker.GetHealth(context.Background())
			if got != tt.want {
				t.Errorf("Checker.GetHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
