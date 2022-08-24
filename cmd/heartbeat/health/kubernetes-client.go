package health

import (
	"context"
	"net/url"
	"path"

	"github.com/m-lab/go/rtx"
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
	ctx       context.Context
	ctxCancel context.CancelFunc
	clientset kubernetes.Interface
}

// MustNewKubernetesClient creates a new KubenernetesClient instance.
// If the client cannot be instantiated, the function will exit.
func MustNewKubernetesClient(ctx context.Context, url *url.URL, pod, node, namespace, auth string) *KubernetesClient {
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

	// Construct a "direct client" using the auth above to contact the API server.
	defClient := clientcmd.NewDefaultClientConfig(
		clusterClient,
		&clientcmd.ConfigOverrides{
			ClusterInfo: api.Cluster{Server: ""},
		},
	)
	restConfig, err := defClient.ClientConfig()
	rtx.Must(err, "failed to create kubernetes config")

	clientset, err := kubernetes.NewForConfig(restConfig)
	rtx.Must(err, "failed to create kubernetes clientset")

	ctx, ctxCancel := context.WithCancel(ctx)
	client := &KubernetesClient{
		pod:       pod,
		node:      node,
		namespace: namespace,
		ctx:       ctx,
		ctxCancel: ctxCancel,
		clientset: clientset,
	}
	return client
}

// isHealthy returns true if the following conditions are true:
//   - The Pod's status is "Running"
//   - The Node's Ready condition is "True"
//   - The Pod does not have a "lame-duck" taint
func (c *KubernetesClient) isHealthy() bool {
	return c.isPodRunning() && c.isNodeReady() && !c.isInMaintenance()
}

func (c *KubernetesClient) isPodRunning() bool {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(c.ctx, c.pod, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.Status.Phase == "Running"
}

func (c *KubernetesClient) isNodeReady() bool {
	node, err := c.clientset.CoreV1().Nodes().Get(c.ctx, c.node, metav1.GetOptions{})
	if err != nil {
		return false
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}

	return false
}

func (c *KubernetesClient) isInMaintenance() bool {
	node, err := c.clientset.CoreV1().Nodes().Get(c.ctx, c.node, metav1.GetOptions{})
	if err != nil {
		return false
	}

	for _, taint := range node.Spec.Taints {
		if taint.Key == "lame-duck" {
			return true
		}
	}

	return false
}
