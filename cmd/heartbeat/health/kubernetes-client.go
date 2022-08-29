package health

import (
	"context"
	"net/url"
	"path"
	"strconv"
	"time"

	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/metrics"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubernetesClient manages requests to the Kubernetes API server.
type KubernetesClient struct {
	pod       string
	node      string
	namespace string
	clientset kubernetes.Interface
}

// MustNewKubernetesClient creates a new KubenernetesClient instance.
// If the client cannot be instantiated, the function will exit.
func MustNewKubernetesClient(url *url.URL, pod, node, namespace, auth string) *KubernetesClient {
	defConfig := getDefaultClientConfig(url, auth)
	restConfig, err := defConfig.ClientConfig()
	rtx.Must(err, "failed to create kubernetes config")

	clientset, err := kubernetes.NewForConfig(restConfig)
	rtx.Must(err, "failed to create kubernetes clientset")

	client := &KubernetesClient{
		pod:       pod,
		node:      node,
		namespace: namespace,
		clientset: clientset,
	}
	return client
}

func getDefaultClientConfig(url *url.URL, auth string) clientcmd.ClientConfig {
	// This is a low-level structure normally created from parsing a kubeconfig
	// file.  Since we know all values we can create the client object directly.
	//
	// The cluster and user names serve only to define a context that
	// associates login credentials with a specific cluster.
	clusterClient := api.Config{
		Clusters: map[string]*api.Cluster{
			// Define the cluster address and CA Certificate.
			"cluster": {
				Server:                url.String(),
				InsecureSkipTLSVerify: false, // Require a valid CA Certificate.
				CertificateAuthority:  path.Join(auth, "ca.crt"),
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			// Define the user credentials for access to the API.
			"user": {
				TokenFile: path.Join(auth, "token"),
			},
		},
		Contexts: map[string]*api.Context{
			// Define a context that refers to the above cluster and user.
			"cluster-user": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		// Use the above context.
		CurrentContext: "cluster-user",
	}

	defConfig := clientcmd.NewDefaultClientConfig(
		clusterClient,
		&clientcmd.ConfigOverrides{
			ClusterInfo: api.Cluster{Server: ""},
		},
	)

	return defConfig
}

// isHealthy returns true if it can determine the following conditions are true:
//   - The Pod's status is "Running"
//   - The Node's Ready condition is "True"
//   - The Node does not have a "lame-duck" taint
//
// OR if it cannot contact the API Server to make a determination.
func (c *KubernetesClient) isHealthy(ctx context.Context) bool {
	start := time.Now()
	isHealthy := c.isPodRunning(ctx) && c.isNodeReady(ctx)
	metrics.KubernetesRequestTimeHistogram.WithLabelValues(strconv.FormatBool(isHealthy)).Observe(time.Since(start).Seconds())
	return isHealthy
}

func (c *KubernetesClient) isPodRunning(ctx context.Context) bool {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, c.pod, metav1.GetOptions{})
	if err != nil {
		metrics.KubernetesRequestsTotal.WithLabelValues(err.Error()).Inc()
		return true
	}

	metrics.KubernetesRequestsTotal.WithLabelValues("OK").Inc()
	return pod.Status.Phase == "Running"
}

// isNodeReady returns true if it can determine the following conditions are true:
//   - The Node's Ready condition is "True"
//   - The Node does not have a "lame-duck" taint
//
// OR if it cannot contact the API Server to make a determination.
func (c *KubernetesClient) isNodeReady(ctx context.Context) bool {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, c.node, metav1.GetOptions{})
	if err != nil {
		metrics.KubernetesRequestsTotal.WithLabelValues(err.Error()).Inc()
		return true
	}

	metrics.KubernetesRequestsTotal.WithLabelValues("OK").Inc()
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return !isInMaintenance(node)
		}
	}

	return false
}

func isInMaintenance(node *v1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == "lame-duck" {
			return true
		}
	}

	return false
}
