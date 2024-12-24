package kube

import (
	"context"
	"os"

	"k8s.io/client-go/tools/clientcmd"

	argorollout "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	csi "sigs.k8s.io/secrets-store-csi-driver/pkg/client/clientset/versioned"
)

// Clients struct exposes interfaces for kubernetes as well as openshift if available
type Clients struct {
	KubernetesClient    kubernetes.Interface
	OpenshiftAppsClient appsclient.Interface
	ArgoRolloutClient   argorollout.Interface
	CSIClient           csi.Interface
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

	var rolloutClient *argorollout.Clientset

	rolloutClient, err = GetArgoRolloutClient()
	if err != nil {
		logrus.Warnf("Unable to create ArgoRollout client error = %v", err)
	}

	var csiClient *csi.Clientset

	csiClient, err = GetCSIClient()
	if err != nil {
		logrus.Warnf("Unable to create CSI client error = %v", err)
	}

	return Clients{
		KubernetesClient:    client,
		OpenshiftAppsClient: appsClient,
		ArgoRolloutClient:   rolloutClient,
		CSIClient:           csiClient,
	}
}

func GetArgoRolloutClient() (*argorollout.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return argorollout.NewForConfig(config)
}

func GetCSIClient() (*csi.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return csi.NewForConfig(config)
}

func isOpenshift() bool {
	client, err := GetKubernetesClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	_, err = client.RESTClient().Get().AbsPath("/apis/project.openshift.io").Do(context.TODO()).Raw()
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
