package handler

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var (
	clients                             = kube.Clients{KubernetesClient: testclient.NewSimpleClientset()}
	namespace                           = "test-handler-" + testutil.RandSeq(5)
	configmapName                       = "testconfigmap-handler-" + testutil.RandSeq(5)
	secretName                          = "testsecret-handler-" + testutil.RandSeq(5)
	projectedConfigMapName              = "testprojectedconfigmap-handler-" + testutil.RandSeq(5)
	projectedSecretName                 = "testprojectedsecret-handler-" + testutil.RandSeq(5)
	configmapWithInitContainer          = "testconfigmapInitContainerhandler-" + testutil.RandSeq(5)
	secretWithInitContainer             = "testsecretWithInitContainer-handler-" + testutil.RandSeq(5)
	projectedConfigMapWithInitContainer = "testProjectedConfigMapWithInitContainer-handler" + testutil.RandSeq(5)
	projectedSecretWithInitContainer    = "testProjectedSecretWithInitContainer-handler" + testutil.RandSeq(5)
	configmapWithInitEnv                = "configmapWithInitEnv-" + testutil.RandSeq(5)
	secretWithInitEnv                   = "secretWithInitEnv-handler-" + testutil.RandSeq(5)
	configmapWithEnvName                = "testconfigmapWithEnv-handler-" + testutil.RandSeq(5)
	configmapWithEnvFromName            = "testconfigmapWithEnvFrom-handler-" + testutil.RandSeq(5)
	secretWithEnvName                   = "testsecretWithEnv-handler-" + testutil.RandSeq(5)
	secretWithEnvFromName               = "testsecretWithEnvFrom-handler-" + testutil.RandSeq(5)
	configmapWithPodAnnotations         = "testconfigmapPodAnnotations-handler-" + testutil.RandSeq(5)
	configmapWithBothAnnotations        = "testconfigmapBothAnnotations-handler-" + testutil.RandSeq(5)
)

func TestMain(m *testing.M) {

	// Creating namespace
	testutil.CreateNamespace(namespace, clients.KubernetesClient)

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
	_, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, projectedConfigMapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, projectedSecretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume in init containers
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, projectedConfigMapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume in init containers
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, projectedSecretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapWithEnvName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretWithEnvName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapWithEnvFromName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretWithInitEnv, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretWithEnvFromName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapWithInitEnv, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapWithPodAnnotations, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating Deployment with configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, configmapWithInitContainer, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, projectedConfigMapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, projectedConfigMapWithInitContainer, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, projectedSecretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, projectedSecretWithInitContainer, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, secretWithInitContainer, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with configmap mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, configmapWithInitEnv, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, secretWithInitEnv, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with env var source as configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap configmap as env var source creation: %v", err)
	}

	// Creating Deployment with env var source as secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, configmapWithEnvFromName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, secretWithEnvFromName, namespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating DaemonSet with configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with configmap in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, projectedConfigMapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, projectedSecretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with env var source as configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap as env var source creation: %v", err)
	}

	// Creating DaemonSet with env var source as secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret creation: %v", err)
	}

	// Creating StatefulSet with configmap in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, projectedConfigMapName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, projectedSecretName, namespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with env var source as configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with env var source as secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretWithEnvName, namespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with pod annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, configmapWithPodAnnotations, namespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with pod annotations: %v", err)
	}

	// Creating Deployment with both annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, configmapWithBothAnnotations, namespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with both annotations: %v", err)
	}
}

func teardown() {
	// Deleting Deployment with configmap
	deploymentError := testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, projectedConfigMapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume mounted in init  container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, projectedConfigMapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, projectedSecretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, projectedSecretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap as env var source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as env var source %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as env var source %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with secret as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with pod annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithPodAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with pod annotations %v", deploymentError)
	}

	// Deleting Deployment with both annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapWithBothAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with both annotations %v", deploymentError)
	}

	// Deleting DaemonSet with configmap
	daemonSetError := testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting DaemonSet with configmap in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, projectedConfigMapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, projectedSecretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting Deployment with configmap as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap as env var source %v", daemonSetError)
	}

	// Deleting Deployment with secret as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret as env var source %v", daemonSetError)
	}

	// Deleting StatefulSet with configmap
	statefulSetError := testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, projectedConfigMapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, projectedSecretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap as env var source %v", statefulSetError)
	}

	// Deleting Deployment with secret as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret as env var source %v", statefulSetError)
	}

	// Deleting Configmap
	err := testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, projectedConfigMapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret used in projected volume
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, projectedSecretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume in init containers
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, projectedConfigMapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Configmap used projected volume in init containers
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, projectedSecretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used in init container
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used in init container %v", err)
	}

	// Deleting Secret used in init container
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used in init container %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source in init container %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source in init container %v", err)
	}

	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapWithPodAnnotations)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with pod annotations: %v", err)
	}

	// Deleting namespace
	testutil.DeleteNamespace(namespace, clients.KubernetesClient)

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

func getCollectors() metrics.Collectors {
	return metrics.NewCollectors()
}

var labelSucceeded = prometheus.Labels{"success": "true"}
var labelFailed = prometheus.Labels{"success": "false"}

func TestRollingUpgradeForDeploymentWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, projectedConfigMapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, projectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotation(t *testing.T) {
	annotatedConfigmapName := "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	configmapObj := testutil.GetConfigmap(namespace, annotatedConfigmapName, "www.google.com")
	configmapObj.Annotations = map[string]string{"test-annotation": "test"}
	configmap, err := clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Create(configmapObj)
	if err != nil {
		t.Errorf("Failed to create config map with annotation.")
	}
	defer clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Delete(configmap.Name, &v1.DeleteOptions{})
	deploymentObj := testutil.GetDeploymentWithEnvVars(namespace, annotatedConfigmapName)
	deploymentObj.Annotations = map[string]string{options.ConfigmapUpdateAutoSearchAnnotation: "test-annotation=test"}
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Create(deploymentObj)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, annotatedConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, annotatedConfigmapName, shaData, "")
	config.SearchAnnotation = options.ConfigmapUpdateAutoSearchAnnotation
	config.ResourceAnnotations = configmap.Annotations
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNoValue(t *testing.T) {
	annotatedConfigmapName := "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	configmapObj := testutil.GetConfigmap(namespace, annotatedConfigmapName, "www.google.com")
	configmapObj.Annotations = map[string]string{"test-annotation": "test"}
	configmap, err := clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Create(configmapObj)
	if err != nil {
		t.Errorf("Failed to create config map with annotation.")
	}
	defer clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Delete(configmap.Name, &v1.DeleteOptions{})
	deploymentObj := testutil.GetDeploymentWithEnvVars(namespace, annotatedConfigmapName)
	deploymentObj.Annotations = map[string]string{options.ConfigmapUpdateAutoSearchAnnotation: "test-annotation"}
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Create(deploymentObj)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, annotatedConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, annotatedConfigmapName, shaData, "")
	config.SearchAnnotation = options.ConfigmapUpdateAutoSearchAnnotation
	config.ResourceAnnotations = configmap.Annotations
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNotFound(t *testing.T) {
	annotatedConfigmapName := "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	configmapObj := testutil.GetConfigmap(namespace, annotatedConfigmapName, "www.google.com")
	configmapObj.Annotations = map[string]string{"test-annotation": "test"}
	configmap, err := clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Create(configmapObj)
	if err != nil {
		t.Errorf("Failed to create config map with annotation.")
	}
	defer clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Delete(configmap.Name, &v1.DeleteOptions{})
	deploymentObj := testutil.GetDeploymentWithEnvVars(namespace, annotatedConfigmapName)
	deploymentObj.Annotations = map[string]string{options.ConfigmapUpdateAutoSearchAnnotation: "test-annotation=not-found"}
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Create(deploymentObj)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, annotatedConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, annotatedConfigmapName, shaData, "")
	config.SearchAnnotation = options.ConfigmapUpdateAutoSearchAnnotation
	config.ResourceAnnotations = configmap.Annotations
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNotMapped(t *testing.T) {
	annotatedConfigmapName := "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	configmapObj := testutil.GetConfigmap(namespace, annotatedConfigmapName, "www.google.com")
	configmapObj.Annotations = map[string]string{"test-annotation": "test"}
	configmap, err := clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Create(configmapObj)
	if err != nil {
		t.Errorf("Failed to create config map with annotation.")
	}
	defer clients.KubernetesClient.CoreV1().ConfigMaps(namespace).Delete(configmap.Name, &v1.DeleteOptions{})
	deploymentObj := testutil.GetDeploymentWithEnvVars(namespace, annotatedConfigmapName+"-different")
	deploymentObj.Annotations = map[string]string{options.ConfigmapUpdateAutoSearchAnnotation: "test-annotation=test"}
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Create(deploymentObj)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, annotatedConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, annotatedConfigmapName, shaData, "")
	config.SearchAnnotation = options.ConfigmapUpdateAutoSearchAnnotation
	config.ResourceAnnotations = configmap.Annotations
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapInInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectVolumeInInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, projectedConfigMapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, projectedConfigMapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvName, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarInInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithInitEnv, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithInitEnv, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarFrom(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvFromName, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvFromName, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, options.SecretUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, projectedSecretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, projectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretinInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolumeinInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, projectedSecretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, projectedSecretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithEnvName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithEnvName, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarFrom(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithEnvFromName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithEnvFromName, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarInInitContainer(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretWithInitEnv, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretWithInitEnv, shaData, options.ReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.facebook.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, daemonSetFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmapInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, projectedConfigMapName, "www.facebook.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, projectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, daemonSetFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDaemonSetWithConfigmapAsEnvVar(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithEnvName, "www.facebook.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithEnvName, shaData, options.ReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, daemonSetFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap used as env var")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDaemonSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, options.SecretUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, daemonSetFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDaemonSetWithSecretInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, projectedSecretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, projectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, daemonSetFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForStatefulSetWithConfigmap(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.twitter.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, statefulSetFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForStatefulSetWithConfigmapInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, projectedConfigMapName, "www.twitter.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, projectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, statefulSetFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForStatefulSetWithSecret(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, secretName, shaData, options.SecretUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, statefulSetFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForStatefulSetWithSecretInProjectedVolume(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, projectedSecretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(constants.SecretEnvVarPostfix, projectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, statefulSetFuncs, collectors)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceUpdate(clients, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestRollingUpgradeForDeploymentWithPodAnnotations(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapWithPodAnnotations, "www.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapWithPodAnnotations, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with pod annotations")
	}

	logrus.Infof("Verifying deployment update")
	envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + constants.ConfigmapEnvVarPostfix
	items := deploymentFuncs.ItemsFunc(clients, config.Namespace)
	var foundPod, foundBoth bool
	for _, i := range items {
		name := util.ToObjectMeta(i).Name
		if name == configmapWithPodAnnotations {
			containers := deploymentFuncs.ContainersFunc(i)
			updated := testutil.GetResourceSHA(containers, envName)
			if updated != config.SHAValue {
				t.Errorf("Deployment was not updated")
			}
			foundPod = true
		}
		if name == configmapWithBothAnnotations {
			containers := deploymentFuncs.ContainersFunc(i)
			updated := testutil.GetResourceSHA(containers, envName)
			if updated == config.SHAValue {
				t.Errorf("Deployment was updated")
			}
			foundBoth = true
		}
	}
	if !foundPod {
		t.Errorf("Deployment with pod annotations was not found")
	}
	if !foundBoth {
		t.Errorf("Deployment with both annotations was not found")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}
}

func TestFailedRollingUpgrade(t *testing.T) {
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "fail.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, configmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	deploymentFuncs.UpdateFunc = func(_ kube.Clients, _ string, _ interface{}) error {
		return fmt.Errorf("error")
	}
	collectors := getCollectors()

	_ = PerformRollingUpgrade(clients, config, deploymentFuncs, collectors)

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelFailed)) != 1 {
		t.Errorf("Counter was not increased")
	}
}
