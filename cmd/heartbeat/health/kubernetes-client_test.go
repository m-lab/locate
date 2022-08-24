package health

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	runningPod = &v1.Pod{
		Status: v1.PodStatus{
			Phase: "Running",
		},
	}
	readyNode = &v1.Node{
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{Type: "Ready", Status: "True"},
			},
		},
	}
	healthyClientset = fake.NewSimpleClientset(
		runningPod, readyNode,
	)
)

func TestKubernetesClient_isHealthy(t *testing.T) {
	tests := []struct {
		name      string
		clientset kubernetes.Interface
		want      bool
	}{
		{
			name:      "healthy",
			clientset: healthyClientset,
			want:      true,
		},
		{
			name: "pod-not-running",
			clientset: fake.NewSimpleClientset(
				&v1.Pod{
					Status: v1.PodStatus{
						Phase: "Pending",
					},
				},
				readyNode,
			),
			want: false,
		},
		{
			name: "node-not-ready",
			clientset: fake.NewSimpleClientset(
				runningPod,
				&v1.Node{
					Status: v1.NodeStatus{
						Conditions: []v1.NodeCondition{
							{Type: "Ready", Status: "False"},
						},
					},
				},
			),
			want: false,
		},
		{
			name: "node-in-maintenance",
			clientset: fake.NewSimpleClientset(
				runningPod,
				&v1.Node{
					Status: v1.NodeStatus{
						Conditions: []v1.NodeCondition{
							{Type: "Ready", Status: "True"},
						},
					},
					Spec: v1.NodeSpec{
						Taints: []v1.Taint{
							{Key: "lame-duck"},
						},
					},
				},
			),
			want: false,
		},
		{
			name: "no-pod",
			clientset: fake.NewSimpleClientset(
				readyNode,
			),
			want: false,
		},
		{
			name: "no-node",
			clientset: fake.NewSimpleClientset(
				runningPod,
			),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubernetesClient{
				clientset: tt.clientset,
			}
			if got := c.isHealthy(); got != tt.want {
				t.Errorf("KubernetesClient.isHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}
