package client

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClientset returns a Kubernetes clientset using default kubeconfig resolution.
func GetClientset() (*kubernetes.Clientset, error) {
	return GetClientsetWithKubeconfig("")
}

// GetClientsetWithKubeconfig returns a Kubernetes clientset for the given kubeconfig path.
// Pass an empty string to use default resolution (in-cluster → KUBECONFIG env → ~/.kube/config).
func GetClientsetWithKubeconfig(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := GetRestConfigWithKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// GetRestConfig returns a REST config using default kubeconfig resolution.
func GetRestConfig() (*rest.Config, error) {
	return GetRestConfigWithKubeconfig("")
}

// GetRestConfigWithKubeconfig returns a REST config for the given kubeconfig path.
// Pass an empty string to use default resolution (in-cluster → KUBECONFIG env → ~/.kube/config).
func GetRestConfigWithKubeconfig(kubeconfig string) (*rest.Config, error) {
	// 1. In-cluster config (when running inside a Pod), only when no explicit path given
	if kubeconfig == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
	}

	// 2. Explicit path supplied by caller
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	// 3. Default ~/.kube/config
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}
