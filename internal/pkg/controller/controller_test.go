package controller

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/helper"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix    = "testsecret-reloader"
)

// Creating a Controller to do a rolling upgrade upon updating the configmap or secret
func TestControllerForUpdatingConfigmapShouldUpdateDeployment(t *testing.T) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Errorf("Unable to create Kubernetes client error = %v", err)
		return
	}
	namespace := "test-reloader"
	logrus.Infof("Step 1: Create namespace")
	helper.CreateNamespace(namespace, client)
	defer helper.DeleteNamespace(namespace, client)

	logrus.Infof("Step 2: Create controller")
	controller, err := NewController(client, "configMaps", namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Step 3: Start controller")
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	configmapName := configmapNamePrefix + "-update-" + helper.RandSeq(5)
	configmapClient := client.CoreV1().ConfigMaps(namespace)

	logrus.Infof("Step 4: Create configmap")
	_, err = configmapClient.Create(helper.GetConfigmap(namespace, configmapName, "www.google.com"))
	if err != nil {
		t.Errorf("Error in configmap creation: %v", err)
	}
	logrus.Infof("Created Configmap %q.\n", configmapName)
	time.Sleep(10 * time.Second)

	logrus.Infof("Step 5: Create Deployment")
	deployment := createDeployment(configmapName, namespace, client)

	logrus.Infof("Step 6: Update configmap for first time")
	logrus.Infof("Updating Configmap %q.\n", configmapName)
	_, err = configmapClient.Get(configmapName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting configmap %v", err)
	}
	_, updateErr := configmapClient.Update(helper.GetConfigmap(namespace, configmapName, "www.stakater.com"))

	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}
	time.Sleep(10 * time.Second)

	logrus.Infof("Step 7: Verify deployment update for first time")
	shaData := helper.ConvertConfigmapToSHA(helper.GetConfigmap(namespace, configmapName, "www.stakater.com"))
	updated := helper.VerifyDeploymentUpdate(client, namespace, configmapName, "_CONFIGMAP", shaData)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(10 * time.Second)

	logrus.Infof("Step 8: Update configmap for Second time")
	_, updateErr = configmapClient.Update(helper.GetConfigmap(namespace, configmapName, "aurorasolutions.io"))
	time.Sleep(10 * time.Second)

	logrus.Infof("Step 9: Verify deployment update for second time")
	shaData = helper.ConvertConfigmapToSHA(helper.GetConfigmap(namespace, configmapName, "aurorasolutions.io"))
	updated = helper.VerifyDeploymentUpdate(client, namespace, configmapName, "_CONFIGMAP", shaData)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(10 * time.Second)

	logrus.Infof("Step 10: Delete Deployment")
	logrus.Infof("Deleting Deployment %q.\n", deployment.GetObjectMeta().GetName())
	deploymentError := controller.client.ExtensionsV1beta1().Deployments(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if deploymentError != nil {
		logrus.Errorf("Error while deleting the configmap %v", deploymentError)
	}

	logrus.Infof("Step 11: Delete Configmap")
	logrus.Infof("Deleting Configmap %q.\n", configmapName)
	err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(15 * time.Second)
}

func createDeployment(deploymentName string, namespace string, client kubernetes.Interface) *v1beta1.Deployment {
	deploymentClient := client.ExtensionsV1beta1().Deployments(namespace)
	deployment := helper.GetDeployment(namespace, deploymentName)
	deployment, err := deploymentClient.Create(deployment)
	if err != nil {
		logrus.Errorf("Error in deployment creation: %v", err)
	}
	logrus.Infof("Created Deployment %q.\n", deployment.GetObjectMeta().GetName())
	return deployment
}

func TestControllerForUpdatingSecretShouldUpdateDeployment(t *testing.T) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Errorf("Unable to create Kubernetes client error = %v", err)
		return
	}
	namespace := "test-reloader-secrets"
	helper.CreateNamespace(namespace, client)
	defer helper.DeleteNamespace(namespace, client)

	controller, err := NewController(client, "secrets", namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	secretName := secretNamePrefix + "-update-" + helper.RandSeq(5)
	secretClient := client.CoreV1().Secrets(namespace)
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = secretClient.Create(helper.GetSecret(namespace, secretName, data))
	if err != nil {
		logrus.Errorf("Error  in secret creation: %v", err)
	}
	logrus.Infof("Created Secret %q.\n", secretName)
	time.Sleep(10 * time.Second)

	logrus.Infof("Updating Secret %q.\n", secretName)
	_, err = secretClient.Get(secretName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting secret %v", err)
	}
	data = "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"
	_, updateErr := secretClient.Update(helper.GetSecret(namespace, secretName, data))

	// TODO: Add functionality to verify reloader functionality here

	if updateErr != nil {
		err := controller.client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the secret %v", err)
		}
		logrus.Errorf("Error while updating the secret %v", err)
	}
	time.Sleep(10 * time.Second)
	logrus.Infof("Deleting Secret %q.\n", secretName)
	err = controller.client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(15 * time.Second)
}
