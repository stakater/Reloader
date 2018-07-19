package handlerTester

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/controller"
	"github.com/stakater/Reloader/internal/pkg/handler"
	helper "github.com/stakater/Reloader/internal/pkg/helper"
	"github.com/stakater/Reloader/pkg/kube"
	v1_beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	client        = getClient()
	namespace     = "test-handler"
	configmapName = "testconfigmap-handler-update-" + helper.RandSeq(5)
	secretName    = "testsecret-handler-update-" + helper.RandSeq(5)
)

func TestMain(m *testing.M) {

	logrus.Infof("Creating namespace %s", namespace)
	helper.CreateNamespace(namespace, client)

	logrus.Infof("Creating controller")
	newController, err := controller.NewController(client, "configMaps", namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}

	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go newController.Run(1, stop)
	time.Sleep(10 * time.Second)

	logrus.Infof("Setting up the test resources")
	setup()

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	logrus.Infof("tearing down the test resources")
	teardown()

	os.Exit(retCode)
}

func getClient() *kubernetes.Clientset {
	newClient, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	return newClient
}

func setup() {
	logrus.Infof("Creating configmap")
	configmapClient := client.CoreV1().ConfigMaps(namespace)
	_, err := configmapClient.Create(helper.GetConfigmap(namespace, configmapName, "www.google.com"))
	if err != nil {
		logrus.Errorf("Error  in configmap creation: %v", err)
	}
	time.Sleep(10 * time.Second)

	logrus.Infof("Creating secret")
	secretClient := client.CoreV1().Secrets(namespace)
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = secretClient.Create(helper.GetSecret(namespace, secretName, data))
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}
	time.Sleep(10 * time.Second)

	logrus.Infof("Creating Deployment with configmap")
	createDeployment(configmapName, namespace)

	logrus.Infof("Creating Deployment with secret")
	createDeployment(secretName, namespace)

	logrus.Infof("Creating Daemonset with configmap")
	createDaemonset(configmapName, namespace)

	logrus.Infof("Creating Daemonset with secret")
	createDaemonset(secretName, namespace)

	logrus.Infof("Creating Statefulset with configmap")
	createStatefulset(configmapName, namespace)

	logrus.Infof("Creating Statefulset with secret")
	createStatefulset(secretName, namespace)

}

func teardown() {
	logrus.Infof("Deleting Deployment with configmap")
	deploymentError := client.ExtensionsV1beta1().Deployments(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	logrus.Infof("Deleting Deployment with secret")
	deploymentError = client.ExtensionsV1beta1().Deployments(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret %v", deploymentError)
	}

	logrus.Infof("Deleting Daemonset with configmap")
	daemonsetError := client.ExtensionsV1beta1().DaemonSets(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if daemonsetError != nil {
		logrus.Errorf("Error while deleting daemonset with configmap %v", daemonsetError)
	}

	logrus.Infof("Deleting Deployment with secret")
	daemonsetError = client.ExtensionsV1beta1().DaemonSets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if daemonsetError != nil {
		logrus.Errorf("Error while deleting daemonset with secret %v", daemonsetError)
	}

	logrus.Infof("Deleting Statefulset with configmap")
	statefulsetError := client.AppsV1beta1().StatefulSets(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if statefulsetError != nil {
		logrus.Errorf("Error while deleting statefulset with configmap %v", statefulsetError)
	}

	logrus.Infof("Deleting Deployment with secret")
	statefulsetError = client.AppsV1beta1().StatefulSets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if statefulsetError != nil {
		logrus.Errorf("Error while deleting statefulset with secret %v", statefulsetError)
	}

	logrus.Infof("Deleting Configmap %q.\n", configmapName)
	err := client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)

	logrus.Infof("Deleting Secret %q.\n", secretName)
	err = client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)

	logrus.Infof("Deleting namespace %q.\n", namespace)
	helper.DeleteNamespace(namespace, client)

}

func createDeployment(deploymentName string, namespace string) *v1beta1.Deployment {
	deploymentClient := client.ExtensionsV1beta1().Deployments(namespace)
	deployment := helper.GetDeployment(namespace, deploymentName)
	deployment, err := deploymentClient.Create(deployment)
	if err != nil {
		logrus.Errorf("Error in deployment creation: %v", err)
	}
	logrus.Infof("Created Deployment %q.\n", deployment.GetObjectMeta().GetName())
	return deployment
}

func createDaemonset(daemonsetName string, namespace string) *v1beta1.DaemonSet {
	daemonsetClient := client.ExtensionsV1beta1().DaemonSets(namespace)
	daemonset := helper.GetDaemonset(namespace, daemonsetName)
	daemonset, err := daemonsetClient.Create(daemonset)
	if err != nil {
		logrus.Errorf("Error in daemonset creation: %v", err)
	}
	logrus.Infof("Created Deployment %q.\n", daemonset.GetObjectMeta().GetName())
	return daemonset
}

func createStatefulset(statefulsetName string, namespace string) *v1_beta1.StatefulSet {
	statefulsetClient := client.AppsV1beta1().StatefulSets(namespace)
	statefulset := helper.GetStatefulset(namespace, statefulsetName)
	statefulset, err := statefulsetClient.Create(statefulset)
	if err != nil {
		logrus.Errorf("Error in statefulset creation: %v", err)
	}
	logrus.Infof("Created Statefulset %q.\n", statefulset.GetObjectMeta().GetName())
	return statefulset
}

func TestRollingUpgradeForDeploymentWithConfigmap(t *testing.T) {
	shaData := helper.ConvertConfigmapToSHA(helper.GetConfigmap(namespace, configmapName, "www.stakater.com"))
	handler.RollingUpgradeForDeployment(client, namespace, configmapName, shaData, "_CONFIGMAP")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying deployment update")
	updated := helper.VerifyDeploymentUpdate(client, namespace, configmapName, "_CONFIGMAP", shaData)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecret(t *testing.T) {
	shaData := helper.ConvertSecretToSHA(helper.GetSecret(namespace, secretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"))
	handler.RollingUpgradeForDeployment(client, namespace, secretName, shaData, "_SECRET")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying deployment update")
	updated := helper.VerifyDeploymentUpdate(client, namespace, secretName, "_SECRET", shaData)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDaemonsetWithConfigmap(t *testing.T) {
	shaData := helper.ConvertConfigmapToSHA(helper.GetConfigmap(namespace, configmapName, "www.facebook.com"))
	handler.RollingUpgradeForDaemonSets(client, namespace, configmapName, shaData, "_CONFIGMAP")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying daemonset update")
	updated := helper.VerifyDaemonsetUpdate(client, namespace, configmapName, "_CONFIGMAP", shaData)
	if !updated {
		t.Errorf("Daemonset was not updated")
	}
}

func TestRollingUpgradeForDaemonsetWithSecret(t *testing.T) {
	shaData := helper.ConvertSecretToSHA(helper.GetSecret(namespace, secretName, "d3d3LmZhY2Vib29rLmNvbQ=="))
	handler.RollingUpgradeForDaemonSets(client, namespace, secretName, shaData, "_SECRET")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying daemonset update")
	updated := helper.VerifyDaemonsetUpdate(client, namespace, secretName, "_SECRET", shaData)
	if !updated {
		t.Errorf("Daemonset was not updated")
	}
}

func TestRollingUpgradeForStatefulsetWithConfigmap(t *testing.T) {
	shaData := helper.ConvertConfigmapToSHA(helper.GetConfigmap(namespace, configmapName, "www.twitter.com"))
	handler.RollingUpgradeForStatefulSets(client, namespace, configmapName, shaData, "_CONFIGMAP")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying statefulset update")
	updated := helper.VerifyStatefulsetUpdate(client, namespace, configmapName, "_CONFIGMAP", shaData)
	if !updated {
		t.Errorf("Statefulset was not updated")
	}
}

func TestRollingUpgradeForStatefulsetWithSecret(t *testing.T) {
	shaData := helper.ConvertSecretToSHA(helper.GetSecret(namespace, secretName, "d3d3LnR3aXR0ZXIuY29t"))
	handler.RollingUpgradeForStatefulSets(client, namespace, secretName, shaData, "_SECRET")
	time.Sleep(5 * time.Second)

	logrus.Infof("Verifying statefulset update")
	updated := helper.VerifyStatefulsetUpdate(client, namespace, secretName, "_SECRET", shaData)
	if !updated {
		t.Errorf("Statefulset was not updated")
	}
}
