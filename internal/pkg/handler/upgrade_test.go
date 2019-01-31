package handler

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var (
	client                   = testclient.NewSimpleClientset()
	namespace                = "test-handler-" + testutil.RandSeq(5)
	configmapName            = "testconfigmap-handler-" + testutil.RandSeq(5)
	secretName               = "testsecret-handler-" + testutil.RandSeq(5)
	configmapWithEnvName     = "testconfigmapWithEnv-handler-" + testutil.RandSeq(3)
	configmapWithEnvFromName = "testconfigmapWithEnvFrom-handler-" + testutil.RandSeq(3)
	secretWithEnvName        = "testsecretWithEnv-handler-" + testutil.RandSeq(5)
	secretWithEnvFromName    = "testsecretWithEnvFrom-handler-" + testutil.RandSeq(5)
)

func TestMain(m *testing.M) {

	// Creating namespace
	testutil.CreateNamespace(namespace, client)

	logrus.Infof("Setting up the test resources")
	setup()

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	logrus.Infof("tearing down the test resources")
	teardown()

	os.Exit(retCode)
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

	_, err = testutil.CreateConfigMap(client, namespace, configmapWithEnvName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(client, namespace, secretWithEnvName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(client, namespace, configmapWithEnvFromName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(client, namespace, secretWithEnvFromName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating Deployment with configmap
	_, err = testutil.CreateDeployment(client, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret
	_, err = testutil.CreateDeployment(client, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with env var source as configmap
	_, err = testutil.CreateDeployment(client, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap configmap as env var source creation: %v", err)
	}

	// Creating Deployment with env var source as secret
	_, err = testutil.CreateDeployment(client, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(client, configmapWithEnvFromName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(client, secretWithEnvFromName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating DaemonSet with configmap
	_, err = testutil.CreateDaemonSet(client, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret
	_, err = testutil.CreateDaemonSet(client, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with env var source as configmap
	_, err = testutil.CreateDaemonSet(client, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap as env var source creation: %v", err)
	}

	// Creating DaemonSet with env var source as secret
	_, err = testutil.CreateDaemonSet(client, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with configmap
	_, err = testutil.CreateStatefulSet(client, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret
	_, err = testutil.CreateStatefulSet(client, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret creation: %v", err)
	}

	// Creating StatefulSet with env var source as configmap
	_, err = testutil.CreateStatefulSet(client, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with env var source as secret
	_, err = testutil.CreateStatefulSet(client, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret configmap as env var source creation: %v", err)
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

	// Deleting Deployment with configmap as env var source
	deploymentError = testutil.DeleteDeployment(client, namespace, configmapWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as env var source %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(client, namespace, secretWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as env var source %v", deploymentError)
	}

	// Deleting Deployment with configmap as envFrom source
	deploymentError = testutil.DeleteDeployment(client, namespace, configmapWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with secret as envFrom source
	deploymentError = testutil.DeleteDeployment(client, namespace, secretWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as envFrom source %v", deploymentError)
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

	// Deleting Deployment with configmap as env var source
	daemonSetError = testutil.DeleteDaemonSet(client, namespace, configmapWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap as env var source %v", daemonSetError)
	}

	// Deleting Deployment with secret as env var source
	daemonSetError = testutil.DeleteDaemonSet(client, namespace, secretWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret as env var source %v", daemonSetError)
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

	// Deleting StatefulSet with configmap as env var source
	statefulSetError = testutil.DeleteStatefulSet(client, namespace, configmapWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap as env var source %v", statefulSetError)
	}

	// Deleting Deployment with secret as env var source
	statefulSetError = testutil.DeleteStatefulSet(client, namespace, secretWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret as env var source %v", statefulSetError)
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

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(client, namespace, configmapWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(client, namespace, secretWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(client, namespace, configmapWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(client, namespace, secretWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting namespace
	testutil.DeleteNamespace(namespace, client)

}

func getConfigWithAnnotations(resourceType string, name string, shaData string, annotation string) util.Config {
	return util.Config{
		Namespace:    namespace,
		ResourceName: name,
		SHAValue:     shaData,
		Annotation:   annotation,
		Type:         resourceType,
	}

}

func TestRollingUpgradeForDeploymentWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvName, shaData, constants.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarFrom(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvFromName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvFromName, shaData, constants.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, constants.SecretUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithEnvName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithEnvName, shaData, constants.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarFrom(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithEnvFromName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithEnvFromName, shaData, constants.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.facebook.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, daemonSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmapAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvName, "www.facebook.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvName, shaData, constants.ReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, daemonSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap used as env var")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
}

func TestRollingUpgradeForDaemonSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, constants.SecretUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, daemonSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
}

func TestRollingUpgradeForStatefulSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.twitter.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, constants.ConfigmapUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, statefulSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
}

func TestRollingUpgradeForStatefulSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, constants.SecretUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()

	err := PerformRollingUpgrade(client, config, statefulSetFuncs)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
}
