package handler

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/common"
	"github.com/stakater/Reloader/internal/pkg/constants"
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

	// Creating DaemonSet with configmap
	_, err = testutil.CreateDaemonSet(client, configmapName, namespace)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret
	_, err = testutil.CreateDaemonSet(client, secretName, namespace)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating StatefulSet with configmap
	_, err = testutil.CreateStatefulSet(client, configmapName, namespace)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret
	_, err = testutil.CreateStatefulSet(client, secretName, namespace)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret creation: %v", err)
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

	// Deleting DaemonSet with configmap
	daemonSetError := testutil.DeleteDaemonSet(client, namespace, configmapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret
	daemonSetError = testutil.DeleteDaemonSet(client, namespace, secretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting StatefulSet with configmap
	statefulSetError := testutil.DeleteStatefulSet(client, namespace, configmapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret
	statefulSetError = testutil.DeleteStatefulSet(client, namespace, secretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
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
	config := Config{
		namespace:    namespace,
		resourceName: configmapName,
		shaValue:     shaData,
		annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetDeploymentItems,
		ContainersFunc: GetDeploymentContainers,
		UpdateFunc:     UpdateDeployment,
	}

	err := PerformRollingUpgrade(client, config, constants.ConfigmapEnvarPostfix, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, configmapName, constants.ConfigmapEnvarPostfix, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := Config{
		namespace:    namespace,
		resourceName: secretName,
		shaValue:     shaData,
		annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetDeploymentItems,
		ContainersFunc: GetDeploymentContainers,
		UpdateFunc:     UpdateDeployment,
	}

	err := PerformRollingUpgrade(client, config, constants.SecretEnvarPostfix, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, secretName, constants.SecretEnvarPostfix, shaData, constants.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.facebook.com")
	config := Config{
		namespace:    namespace,
		resourceName: configmapName,
		shaValue:     shaData,
		annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetDaemonSetItems,
		ContainersFunc: GetDaemonSetContainers,
		UpdateFunc:     UpdateDaemonSet,
	}

	err := PerformRollingUpgrade(client, config, constants.ConfigmapEnvarPostfix, daemonSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, configmapName, constants.ConfigmapEnvarPostfix, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
}

func TestRollingUpgradeForDaemonSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LmZhY2Vib29rLmNvbQ==")

	config := Config{
		namespace:    namespace,
		resourceName: secretName,
		shaValue:     shaData,
		annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetDaemonSetItems,
		ContainersFunc: GetDaemonSetContainers,
		UpdateFunc:     UpdateDaemonSet,
	}

	err := PerformRollingUpgrade(client, config, constants.SecretEnvarPostfix, daemonSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, secretName, constants.SecretEnvarPostfix, shaData, constants.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
}

func TestRollingUpgradeForStatefulSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.twitter.com")

	config := Config{
		namespace:    namespace,
		resourceName: configmapName,
		shaValue:     shaData,
		annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetStatefulSetItems,
		ContainersFunc: GetStatefulsetContainers,
		UpdateFunc:     UpdateStatefulset,
	}

	err := PerformRollingUpgrade(client, config, constants.ConfigmapEnvarPostfix, statefulSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, configmapName, constants.ConfigmapEnvarPostfix, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
}

func TestRollingUpgradeForStatefulSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LnR3aXR0ZXIuY29t")

	config := Config{
		namespace:    namespace,
		resourceName: secretName,
		shaValue:     shaData,
		annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := RollingUpgradeFuncs{
		ItemsFunc:      GetStatefulSetItems,
		ContainersFunc: GetStatefulsetContainers,
		UpdateFunc:     UpdateStatefulset,
	}

	err := PerformRollingUpgrade(client, config, constants.SecretEnvarPostfix, statefulSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, secretName, constants.SecretEnvarPostfix, shaData, constants.SecretUpdateOnChangeAnnotation)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
}
