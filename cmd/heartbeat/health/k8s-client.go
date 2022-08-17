package health

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesClient ...
type KubernetesClient struct {
	pod  *v1.Pod
	node *v1.Node
}

// NewKubernetesClient ...
func NewKubernetesClient(podName, nodeName, namespace string) (*KubernetesClient, error) {
	client := &KubernetesClient{
		pod:  &v1.Pod{},
		node: &v1.Node{},
	}

	// Creates the in-cluster config.
	config, err := rest.InClusterConfig()
	if err != nil {
		return client, err
	}
	fmt.Printf("%+v\n", config)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return client, err
	}
	fmt.Printf("%+v\n", clientset)

	client.node, err = clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return client, err
	}

	client.pod, err = clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	return client, err
}

func (kc *KubernetesClient) isHealthy() bool {
	return kc.isPodRunning() && kc.isNodeReady() && !kc.isInMaintenance()
}

func (kc *KubernetesClient) isPodRunning() bool {
	return kc.pod.Status.Phase == "Running"
}

func (kc *KubernetesClient) isNodeReady() bool {
	for _, condition := range kc.node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func (kc *KubernetesClient) isInMaintenance() bool {
	for _, taint := range kc.node.Spec.Taints {
		if taint.Key == "lame-duck" {
			return true
		}
	}
	return false
}
