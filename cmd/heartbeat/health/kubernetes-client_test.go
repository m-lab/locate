package health

import (
	"context"
	"net/url"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/testingx"
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

func TestKubernetesClient_MustNewKubernetesClient(t *testing.T) {
	u := &url.URL{
		Scheme: "https",
		Host:   "localhost:1234",
	}
	defConfig := getDefaultClientConfig(u, "testdata/")
	restConfig, err := defConfig.ClientConfig()
	testingx.Must(t, err, "failed to create kubernetes config")
	clientset, err := kubernetes.NewForConfig(restConfig)
	testingx.Must(t, err, "failed to create kubernetes clientset")

	want := &KubernetesClient{
		pod:       "pod",
		node:      "node",
		namespace: "namespace",
		clientset: clientset,
	}

	got := MustNewKubernetesClient(u, "pod", "node", "namespace", "testdata/")

	if diff := deep.Equal(got, want); diff != nil {
		t.Errorf("MustNewKubernetesClient() got: %+v, want:: %+v", got, want)
	}
}

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
			name: "pod-call-fail",
			clientset: fake.NewSimpleClientset(
				readyNode,
			),
			want: true,
		},
		{
			name: "node-call-fail",
			clientset: fake.NewSimpleClientset(
				runningPod,
			),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &KubernetesClient{
				clientset: tt.clientset,
			}
			if got := c.isHealthy(context.Background()); got != tt.want {
				t.Errorf("KubernetesClient.isHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}
