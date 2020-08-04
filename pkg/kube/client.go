package kube

import (
	"os"

	"k8s.io/client-go/tools/clientcmd"

	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Clients struct exposes interfaces for kubernetes as well as openshift if available
type Clients struct {
	KubernetesClient    kubernetes.Interface
	OpenshiftAppsClient appsclient.Interface
}

var (
	// IsOpenshift is true if environment is Openshift, it is false if environment is Kubernetes
	IsOpenshift = isOpenshift()
)

// GetClients returns a `Clients` object containing both openshift and kubernetes clients with an openshift identifier
func GetClients() Clients {
	client, err := GetKubernetesClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	var appsClient *appsclient.Clientset

	if IsOpenshift {
		appsClient, err = GetOpenshiftAppsClient()
		if err != nil {
			logrus.Warnf("Unable to create Openshift Apps client error = %v", err)
		}
	}

	return Clients{
		KubernetesClient:    client,
		OpenshiftAppsClient: appsClient,
	}
}

func isOpenshift() bool {
	client, err := GetKubernetesClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	_, err = client.RESTClient().Get().AbsPath("/apis/project.openshift.io").Do().Raw()
	if err == nil {
		logrus.Info("Environment: Openshift")
		return true
	}
	logrus.Info("Environment: Kubernetes")
	return false
}

// GetOpenshiftAppsClient returns an Openshift Client that can query on Apps
func GetOpenshiftAppsClient() (*appsclient.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return appsclient.NewForConfig(config)
}

// GetKubernetesClient gets the client for k8s, if ~/.kube/config exists so get that config else incluster config
func GetKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func getConfig() (*rest.Config, error) {
	var config *rest.Config
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

	return config, nil
}
