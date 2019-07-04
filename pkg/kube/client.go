package kube

import (
	"errors"
	"os"

	"k8s.io/client-go/tools/clientcmd"

	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client struct exposes interfaces for kubernetes as well as openshift if available
type Clients struct {
	KubernetesClient    kubernetes.Interface
	OpenshiftAppsClient appsclient.Interface
	IsOpenshift         bool
}

// GetClients returns a `Clients` object containing both openshift and kubernetes clients with an openshift identifier
func GetClients() Clients {
	client, err := GetClient()
	isOpenshift := true
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}

	appsClient, err := GetOpenshiftAppsClient()
	if err != nil {
		logrus.Warnf("Unable to create Openshift Apps client error = %v", err)
		isOpenshift = false
	}
	return Clients{
		KubernetesClient:    client,
		OpenshiftAppsClient: appsClient,
		IsOpenshift:         isOpenshift,
	}
}

func isOpenshift() bool {
	client, err := GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	_, err = client.RESTClient().Get().AbsPath("/apis/project.openshift.io").Do().Raw()
	if err == nil {
		return true
	}
	return false
}

// GetOpenshiftAppsClient returns an Openshift Client that can query on Apps
func GetOpenshiftAppsClient() (*appsclient.Clientset, error) {
	if !isOpenshift() {
		return nil, errors.New("Not running on Openshift")
	}
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return appsclient.NewForConfig(config)
}

// GetClient gets the client for k8s, if ~/.kube/config exists so get that config else incluster config
func GetClient() (*kubernetes.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func getConfig() (*rest.Config, error) {
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

	return config, nil
}
