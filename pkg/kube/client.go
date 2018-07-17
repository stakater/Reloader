package kube

import (
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GetClient gets the client for k8s, if ~/.kube/config exists so get that config else incluster config
func GetClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("HOME") + "/.kube/config"
	}
	//If file exists so use that config settings
	if _, err := os.Stat(kubeconfigPath); err == nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, err
		}
	} else { //Use Incluster Configuration
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
