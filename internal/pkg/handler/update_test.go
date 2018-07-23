package handler

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/common"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/client-go/kubernetes"
)

var (
	client        = getClient()
	namespace     = "test-handler"
	configmapName = "testconfigmap-handler-update-" + common.RandSeq(5)
	secretName    = "testsecret-handler-update-" + common.RandSeq(5)
)

func TestMain(m *testing.M) {

	logrus.Infof("Creating namespace %s", namespace)
	testutil.CreateNamespace(namespace, client)

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
	// Creating configmap
	_, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating Deployment with configmap
	_, err = testutil.CreateDeployment(client, configmapName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret
	_, err = testutil.CreateDeployment(client, secretName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Daemonset with configmap
	_, err = testutil.CreateDaemonset(client, configmapName, namespace)
	if err != nil {
		logrus.Errorf("Error in Daemonset with configmap creation: %v", err)
	}

	// Creating Daemonset with secret
	_, err = testutil.CreateDaemonset(client, secretName, namespace)
	if err != nil {
		logrus.Errorf("Error in Daemonset with secret creation: %v", err)
	}

	// Creating Statefulset with configmap
	_, err = testutil.CreateStatefulset(client, configmapName, namespace)
	if err != nil {
		logrus.Errorf("Error in Statefulset with configmap creation: %v", err)
	}

	// Creating Statefulset with secret
	_, err = testutil.CreateStatefulset(client, secretName, namespace)
	if err != nil {
		logrus.Errorf("Error in Statefulset with secret creation: %v", err)
	}

}

func teardown() {
	// Deleting Deployment with configmap
	deploymentError := testutil.DeleteDeployment(client, namespace, configmapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(client, namespace, secretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret %v", deploymentError)
	}

	// Deleting Daemonset with configmap
	daemonsetError := testutil.DeleteDaemonset(client, namespace, configmapName)
	if daemonsetError != nil {
		logrus.Errorf("Error while deleting daemonset with configmap %v", daemonsetError)
	}

	// Deleting Deployment with secret
	daemonsetError = testutil.DeleteDaemonset(client, namespace, secretName)
	if daemonsetError != nil {
		logrus.Errorf("Error while deleting daemonset with secret %v", daemonsetError)
	}

	// Deleting Statefulset with configmap
	statefulsetError := testutil.DeleteStatefulset(client, namespace, configmapName)
	if statefulsetError != nil {
		logrus.Errorf("Error while deleting statefulset with configmap %v", statefulsetError)
	}

	// Deleting Deployment with secret
	statefulsetError = testutil.DeleteStatefulset(client, namespace, secretName)
	if statefulsetError != nil {
		logrus.Errorf("Error while deleting statefulset with secret %v", statefulsetError)
	}

	// Deleting Configmap
	err := testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting namespace
	testutil.DeleteNamespace(namespace, client)

}

func TestRollingUpgradeForDeploymentWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, configmapName, "www.stakater.com")
	err := RollingUpgradeDeployment(client, namespace, configmapName, shaData, common.ConfigmapEnvarPostfix, common.ConfigmapUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	err := RollingUpgradeDeployment(client, namespace, secretName, shaData, common.SecretEnvarPostfix, common.SecretUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDaemonsetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.facebook.com")
	err := RollingUpgradeDaemonSets(client, namespace, configmapName, shaData, common.ConfigmapEnvarPostfix, common.ConfigmapUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonset update")
	updated := testutil.VerifyDaemonsetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Daemonset was not updated")
	}
}

func TestRollingUpgradeForDaemonsetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	err := RollingUpgradeDaemonSets(client, namespace, secretName, shaData, common.SecretEnvarPostfix, common.SecretUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonset update")
	updated := testutil.VerifyDaemonsetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Daemonset was not updated")
	}
}

func TestRollingUpgradeForStatefulsetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.twitter.com")
	err := RollingUpgradeStatefulSets(client, namespace, configmapName, shaData, common.ConfigmapEnvarPostfix, common.ConfigmapUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulset update")
	updated := testutil.VerifyStatefulsetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Statefulset was not updated")
	}
}

func TestRollingUpgradeForStatefulsetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LnR3aXR0ZXIuY29t")
	err := RollingUpgradeStatefulSets(client, namespace, secretName, shaData, common.SecretEnvarPostfix, common.SecretUpdateOnChangeAnnotation)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulset update")
	updated := testutil.VerifyStatefulsetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Statefulset was not updated")
	}
}
