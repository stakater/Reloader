package handler

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/metrics"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	patchtypes "k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
)

var (
	clients = kube.Clients{KubernetesClient: testclient.NewSimpleClientset()}

	arsNamespace                               = "test-handler-" + testutil.RandSeq(5)
	arsConfigmapName                           = "testconfigmap-handler-" + testutil.RandSeq(5)
	arsSecretName                              = "testsecret-handler-" + testutil.RandSeq(5)
	arsProjectedConfigMapName                  = "testprojectedconfigmap-handler-" + testutil.RandSeq(5)
	arsProjectedSecretName                     = "testprojectedsecret-handler-" + testutil.RandSeq(5)
	arsConfigmapWithInitContainer              = "testconfigmapInitContainerhandler-" + testutil.RandSeq(5)
	arsSecretWithInitContainer                 = "testsecretWithInitContainer-handler-" + testutil.RandSeq(5)
	arsProjectedConfigMapWithInitContainer     = "testProjectedConfigMapWithInitContainer-handler" + testutil.RandSeq(5)
	arsProjectedSecretWithInitContainer        = "testProjectedSecretWithInitContainer-handler" + testutil.RandSeq(5)
	arsConfigmapWithInitEnv                    = "configmapWithInitEnv-" + testutil.RandSeq(5)
	arsSecretWithInitEnv                       = "secretWithInitEnv-handler-" + testutil.RandSeq(5)
	arsConfigmapWithEnvName                    = "testconfigmapWithEnv-handler-" + testutil.RandSeq(5)
	arsConfigmapWithEnvFromName                = "testconfigmapWithEnvFrom-handler-" + testutil.RandSeq(5)
	arsSecretWithEnvName                       = "testsecretWithEnv-handler-" + testutil.RandSeq(5)
	arsSecretWithEnvFromName                   = "testsecretWithEnvFrom-handler-" + testutil.RandSeq(5)
	arsConfigmapWithPodAnnotations             = "testconfigmapPodAnnotations-handler-" + testutil.RandSeq(5)
	arsConfigmapWithBothAnnotations            = "testconfigmapBothAnnotations-handler-" + testutil.RandSeq(5)
	arsConfigmapAnnotated                      = "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	arsConfigMapWithNonAnnotatedDeployment     = "testconfigmapNonAnnotatedDeployment-handler-" + testutil.RandSeq(5)
	arsSecretWithSecretAutoAnnotation          = "testsecretwithsecretautoannotationdeployment-handler-" + testutil.RandSeq(5)
	arsConfigmapWithConfigMapAutoAnnotation    = "testconfigmapwithconfigmapautoannotationdeployment-handler-" + testutil.RandSeq(5)
	arsSecretWithExcludeSecretAnnotation       = "testsecretwithsecretexcludeannotationdeployment-handler-" + testutil.RandSeq(5)
	arsConfigmapWithExcludeConfigMapAnnotation = "testconfigmapwithconfigmapexcludeannotationdeployment-handler-" + testutil.RandSeq(5)
	arsConfigmapWithPausedDeployment           = "testconfigmapWithPausedDeployment-handler-" + testutil.RandSeq(5)
	arsConfigmapWithIgnoreAnnotation           = "testconfigmapWithIgnoreAnnotation-handler-" + testutil.RandSeq(5)
	arsSecretWithIgnoreAnnotation              = "testsecretWithIgnoreAnnotation-handler-" + testutil.RandSeq(5)

	ersNamespace                               = "test-handler-" + testutil.RandSeq(5)
	ersConfigmapName                           = "testconfigmap-handler-" + testutil.RandSeq(5)
	ersSecretName                              = "testsecret-handler-" + testutil.RandSeq(5)
	ersProjectedConfigMapName                  = "testprojectedconfigmap-handler-" + testutil.RandSeq(5)
	ersProjectedSecretName                     = "testprojectedsecret-handler-" + testutil.RandSeq(5)
	ersConfigmapWithInitContainer              = "testconfigmapInitContainerhandler-" + testutil.RandSeq(5)
	ersSecretWithInitContainer                 = "testsecretWithInitContainer-handler-" + testutil.RandSeq(5)
	ersProjectedConfigMapWithInitContainer     = "testProjectedConfigMapWithInitContainer-handler" + testutil.RandSeq(5)
	ersProjectedSecretWithInitContainer        = "testProjectedSecretWithInitContainer-handler" + testutil.RandSeq(5)
	ersConfigmapWithInitEnv                    = "configmapWithInitEnv-" + testutil.RandSeq(5)
	ersSecretWithInitEnv                       = "secretWithInitEnv-handler-" + testutil.RandSeq(5)
	ersConfigmapWithEnvName                    = "testconfigmapWithEnv-handler-" + testutil.RandSeq(5)
	ersConfigmapWithEnvFromName                = "testconfigmapWithEnvFrom-handler-" + testutil.RandSeq(5)
	ersSecretWithEnvName                       = "testsecretWithEnv-handler-" + testutil.RandSeq(5)
	ersSecretWithEnvFromName                   = "testsecretWithEnvFrom-handler-" + testutil.RandSeq(5)
	ersConfigmapWithPodAnnotations             = "testconfigmapPodAnnotations-handler-" + testutil.RandSeq(5)
	ersConfigmapWithBothAnnotations            = "testconfigmapBothAnnotations-handler-" + testutil.RandSeq(5)
	ersConfigmapAnnotated                      = "testconfigmapAnnotated-handler-" + testutil.RandSeq(5)
	ersSecretWithSecretAutoAnnotation          = "testsecretwithsecretautoannotationdeployment-handler-" + testutil.RandSeq(5)
	ersConfigmapWithConfigMapAutoAnnotation    = "testconfigmapwithconfigmapautoannotationdeployment-handler-" + testutil.RandSeq(5)
	ersSecretWithSecretExcludeAnnotation       = "testsecretwithsecretexcludeannotationdeployment-handler-" + testutil.RandSeq(5)
	ersConfigmapWithConfigMapExcludeAnnotation = "testconfigmapwithconfigmapexcludeannotationdeployment-handler-" + testutil.RandSeq(5)
	ersConfigmapWithPausedDeployment           = "testconfigmapWithPausedDeployment-handler-" + testutil.RandSeq(5)
	ersConfigmapWithIgnoreAnnotation           = "testconfigmapWithIgnoreAnnotation-handler-" + testutil.RandSeq(5)
	ersSecretWithIgnoreAnnotation              = "testsecretWithIgnoreAnnotation-handler-" + testutil.RandSeq(5)
)

func TestMain(m *testing.M) {

	// Creating namespaces
	testutil.CreateNamespace(arsNamespace, clients.KubernetesClient)
	testutil.CreateNamespace(ersNamespace, clients.KubernetesClient)

	logrus.Infof("Setting up the annotation reload strategy test resources")
	setupArs()
	logrus.Infof("Setting up the env-var reload strategy test resources")
	setupErs()

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	logrus.Infof("tearing down the annotation reload strategy test resources")
	teardownArs()
	logrus.Infof("tearing down the env-var reload strategy test resources")
	teardownErs()

	os.Exit(retCode)
}

func setupArs() {
	// Creating configmap
	_, err := testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsProjectedSecretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume in init containers
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume in init containers
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsProjectedSecretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithEnvName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvFromName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithInitEnv, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithEnvFromName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitEnv, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithPodAnnotations, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigMapWithNonAnnotatedDeployment, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret used with secret auto annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithSecretAutoAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap used with configmap auto annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithConfigMapAutoAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating configmap for testing pausing deployments
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithPausedDeployment, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret used with secret auto annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithExcludeSecretAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap used with configmap auto annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithExcludeConfigMapAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating configmap with ignore annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithIgnoreAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}
	// Patch with ignore annotation
	cmClient := clients.KubernetesClient.CoreV1().ConfigMaps(arsNamespace)
	patch := []byte(`{"metadata":{"annotations":{"reloader.stakater.com/ignore":"true"}}}`)
	_, _ = cmClient.Patch(context.TODO(), arsConfigmapWithIgnoreAnnotation, patchtypes.MergePatchType, patch, metav1.PatchOptions{})

	// Creating secret with ignore annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, arsNamespace, arsSecretWithIgnoreAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}
	secretClient := clients.KubernetesClient.CoreV1().Secrets(arsNamespace)
	_, _ = secretClient.Patch(context.TODO(), arsSecretWithIgnoreAnnotation, patchtypes.MergePatchType, patch, metav1.PatchOptions{})

	// Creating Deployment referencing configmap with ignore annotation
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsConfigmapWithIgnoreAnnotation, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap ignore annotation creation: %v", err)
	}
	// Creating Deployment referencing secret with ignore annotation
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsSecretWithIgnoreAnnotation, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret ignore annotation creation: %v", err)
	}

	// Creating Deployment with configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsConfigmapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsConfigmapWithInitContainer, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsProjectedConfigMapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsProjectedConfigMapWithInitContainer, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsProjectedSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsProjectedSecretWithInitContainer, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsSecretWithInitContainer, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with configmap mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsConfigmapWithInitEnv, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, arsSecretWithInitEnv, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with env var source as configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsConfigmapWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap configmap as env var source creation: %v", err)
	}

	// Creating Deployment with env var source as secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, arsSecretWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, arsConfigmapWithEnvFromName, arsNamespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, arsSecretWithEnvFromName, arsNamespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSourceAndAnnotations(
		clients.KubernetesClient,
		arsConfigmapAnnotated,
		arsNamespace,
		map[string]string{"reloader.stakater.com/search": "true"},
	)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with configmap and without annotations
	_, err = testutil.CreateDeploymentWithEnvVarSourceAndAnnotations(clients.KubernetesClient, arsConfigMapWithNonAnnotatedDeployment, arsNamespace, map[string]string{})
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap and without annotation creation: %v", err)
	}

	// Creating Deployment with secret and with secret auto annotation
	_, err = testutil.CreateDeploymentWithTypedAutoAnnotation(clients.KubernetesClient, arsSecretWithSecretAutoAnnotation, arsNamespace, testutil.SecretResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret and with secret auto annotation: %v", err)
	}

	// Creating Deployment with secret and with secret auto annotation
	_, err = testutil.CreateDeploymentWithTypedAutoAnnotation(clients.KubernetesClient, arsConfigmapWithConfigMapAutoAnnotation, arsNamespace, testutil.ConfigmapResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap and with configmap auto annotation: %v", err)
	}

	// Creating Deployment with secret and exclude secret annotation
	_, err = testutil.CreateDeploymentWithExcludeAnnotation(clients.KubernetesClient, arsSecretWithExcludeSecretAnnotation, arsNamespace, testutil.SecretResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret and with secret exclude annotation: %v", err)
	}

	// Creating Deployment with secret and exclude configmap annotation
	_, err = testutil.CreateDeploymentWithExcludeAnnotation(clients.KubernetesClient, arsConfigmapWithExcludeConfigMapAnnotation, arsNamespace, testutil.ConfigmapResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap and with configmap exclude annotation: %v", err)
	}

	// Creating DaemonSet with configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsConfigmapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with configmap in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsProjectedConfigMapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsProjectedSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with env var source as configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsConfigmapWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap as env var source creation: %v", err)
	}

	// Creating DaemonSet with env var source as secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, arsSecretWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsConfigmapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret creation: %v", err)
	}

	// Creating StatefulSet with configmap in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsProjectedConfigMapName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsProjectedSecretName, arsNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with env var source as configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsConfigmapWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with env var source as secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, arsSecretWithEnvName, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with pod annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, arsConfigmapWithPodAnnotations, arsNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with pod annotations: %v", err)
	}

	// Creating Deployment with both annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, arsConfigmapWithBothAnnotations, arsNamespace, true)

	if err != nil {
		logrus.Errorf("Error in Deployment with both annotations: %v", err)
	}

	// Creating Deployment with pause annotation
	_, err = testutil.CreateDeploymentWithAnnotations(clients.KubernetesClient, arsConfigmapWithPausedDeployment, arsNamespace, map[string]string{options.PauseDeploymentAnnotation: "10s"}, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}
}

func teardownArs() {
	// Deleting Deployment with configmap
	deploymentError := testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume mounted in init  container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsProjectedSecretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsProjectedSecretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap as env var source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as env var source %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as env var source %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with secret as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with pod annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithPodAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with pod annotations %v", deploymentError)
	}

	// Deleting Deployment with both annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithBothAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with both annotations %v", deploymentError)
	}

	// Deleting Deployment with search annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapAnnotated)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with search annotation %v", deploymentError)
	}

	// Deleting Deployment with secret and secret auto annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithSecretAutoAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret auto annotation %v", deploymentError)
	}

	// Deleting Deployment with configmap and configmap auto annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithConfigMapAutoAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap auto annotation %v", deploymentError)
	}

	// Deleting Deployment with secret and exclude secret annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsSecretWithExcludeSecretAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret auto annotation %v", deploymentError)
	}

	// Deleting Deployment with configmap and exclude configmap annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithExcludeConfigMapAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap auto annotation %v", deploymentError)
	}

	// Deleting DaemonSet with configmap
	daemonSetError := testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsConfigmapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsSecretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting DaemonSet with configmap in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsProjectedSecretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting Deployment with configmap as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap as env var source %v", daemonSetError)
	}

	// Deleting Deployment with secret as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, arsNamespace, arsSecretWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret as env var source %v", daemonSetError)
	}

	// Deleting StatefulSet with configmap
	statefulSetError := testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsConfigmapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsSecretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsProjectedSecretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap as env var source %v", statefulSetError)
	}

	// Deleting Deployment with secret as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, arsNamespace, arsSecretWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret as env var source %v", statefulSetError)
	}

	// Deleting Deployment with pasuse annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, arsNamespace, arsConfigmapWithPausedDeployment)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Configmap
	err := testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret used in projected volume
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsProjectedSecretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume in init containers
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsProjectedConfigMapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Configmap used projected volume in init containers
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsProjectedSecretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used in init container
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used in init container %v", err)
	}

	// Deleting Secret used in init container
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used in init container %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source in init container %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source in init container %v", err)
	}

	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithPodAnnotations)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with pod annotations: %v", err)
	}

	// Deleting Secret used with secret auto annotation
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithSecretAutoAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used with secret auto annotations: %v", err)
	}

	// Deleting ConfigMap used with configmap auto annotation
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithConfigMapAutoAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with configmap auto annotations: %v", err)
	}

	// Deleting Secret used with exclude secret annotation
	err = testutil.DeleteSecret(clients.KubernetesClient, arsNamespace, arsSecretWithExcludeSecretAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used with secret auto annotations: %v", err)
	}

	// Deleting ConfigMap used with exclude configmap annotation
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithExcludeConfigMapAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with configmap auto annotations: %v", err)
	}

	// Deleting configmap for testing pausing deployments
	err = testutil.DeleteConfigMap(clients.KubernetesClient, arsNamespace, arsConfigmapWithPausedDeployment)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap: %v", err)
	}

	// Deleting namespace
	testutil.DeleteNamespace(arsNamespace, clients.KubernetesClient)

}

func setupErs() {
	// Creating configmap
	_, err := testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	data := "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersProjectedSecretName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap will be used in projected volume in init containers
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret will be used in projected volume in init containers
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersProjectedSecretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithEnvName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvFromName, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating configmap for testing pausing deployments
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithPausedDeployment, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithInitEnv, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitContainer, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithEnvFromName, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitEnv, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithInitContainer, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithPodAnnotations, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret used with secret auto annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithSecretAutoAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap used with configmap auto annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapAutoAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating secret used with secret exclude annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithSecretExcludeAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}

	// Creating configmap used with configmap exclude annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapExcludeAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}

	// Creating configmap with ignore annotation
	_, err = testutil.CreateConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithIgnoreAnnotation, "www.google.com")
	if err != nil {
		logrus.Errorf("Error in configmap creation: %v", err)
	}
	cmClient := clients.KubernetesClient.CoreV1().ConfigMaps(ersNamespace)
	patch := []byte(`{"metadata":{"annotations":{"reloader.stakater.com/ignore":"true"}}}`)
	_, _ = cmClient.Patch(context.TODO(), ersConfigmapWithIgnoreAnnotation, patchtypes.MergePatchType, patch, metav1.PatchOptions{})

	// Creating secret with ignore annotation
	_, err = testutil.CreateSecret(clients.KubernetesClient, ersNamespace, ersSecretWithIgnoreAnnotation, data)
	if err != nil {
		logrus.Errorf("Error in secret creation: %v", err)
	}
	secretClient := clients.KubernetesClient.CoreV1().Secrets(ersNamespace)
	_, _ = secretClient.Patch(context.TODO(), ersSecretWithIgnoreAnnotation, patchtypes.MergePatchType, patch, metav1.PatchOptions{})

	// Creating Deployment referencing configmap with ignore annotation
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersConfigmapWithIgnoreAnnotation, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap ignore annotation creation: %v", err)
	}
	// Creating Deployment referencing secret with ignore annotation
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersSecretWithIgnoreAnnotation, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret ignore annotation creation: %v", err)
	}

	// Creating Deployment with configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersConfigmapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersConfigmapWithInitContainer, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersProjectedConfigMapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with configmap in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersProjectedConfigMapWithInitContainer, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret in projected volume
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersProjectedSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret in projected volume mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersProjectedSecretWithInitContainer, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret mounted in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersSecretWithInitContainer, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with configmap mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersConfigmapWithInitEnv, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating Deployment with secret mounted as Env in init container
	_, err = testutil.CreateDeploymentWithInitContainer(clients.KubernetesClient, ersSecretWithInitEnv, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret creation: %v", err)
	}

	// Creating Deployment with env var source as configmap
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersConfigmapWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap configmap as env var source creation: %v", err)
	}

	// Creating Deployment with env var source as secret
	_, err = testutil.CreateDeployment(clients.KubernetesClient, ersSecretWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, ersConfigmapWithEnvFromName, ersNamespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSource(clients.KubernetesClient, ersSecretWithEnvFromName, ersNamespace)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with envFrom source as secret
	_, err = testutil.CreateDeploymentWithEnvVarSourceAndAnnotations(
		clients.KubernetesClient,
		ersConfigmapAnnotated,
		ersNamespace,
		map[string]string{"reloader.stakater.com/search": "true"},
	)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret configmap as envFrom source creation: %v", err)
	}

	// Creating Deployment with secret and with secret auto annotation
	_, err = testutil.CreateDeploymentWithTypedAutoAnnotation(clients.KubernetesClient, ersSecretWithSecretAutoAnnotation, ersNamespace, testutil.SecretResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret and with secret auto annotation: %v", err)
	}

	// Creating Deployment with secret and with secret auto annotation
	_, err = testutil.CreateDeploymentWithTypedAutoAnnotation(clients.KubernetesClient, ersConfigmapWithConfigMapAutoAnnotation, ersNamespace, testutil.ConfigmapResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap and with configmap auto annotation: %v", err)
	}

	// Creating Deployment with secret and with secret exclude annotation
	_, err = testutil.CreateDeploymentWithExcludeAnnotation(clients.KubernetesClient, ersSecretWithSecretExcludeAnnotation, ersNamespace, testutil.SecretResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with secret and with secret exclude annotation: %v", err)
	}

	// Creating Deployment with secret and with secret exclude annotation
	_, err = testutil.CreateDeploymentWithExcludeAnnotation(clients.KubernetesClient, ersConfigmapWithConfigMapExcludeAnnotation, ersNamespace, testutil.ConfigmapResourceType)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap and with configmap exclude annotation: %v", err)
	}

	// Creating Deployment with pause annotation
	_, err = testutil.CreateDeploymentWithAnnotations(clients.KubernetesClient, ersConfigmapWithPausedDeployment, ersNamespace, map[string]string{options.PauseDeploymentAnnotation: "10s"}, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with configmap creation: %v", err)
	}

	// Creating DaemonSet with configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersConfigmapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with configmap in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersProjectedConfigMapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap creation: %v", err)
	}

	// Creating DaemonSet with secret in projected volume
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersProjectedSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret creation: %v", err)
	}

	// Creating DaemonSet with env var source as configmap
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersConfigmapWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with configmap as env var source creation: %v", err)
	}

	// Creating DaemonSet with env var source as secret
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, ersSecretWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in DaemonSet with secret configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersConfigmapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret creation: %v", err)
	}

	// Creating StatefulSet with configmap in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersProjectedConfigMapName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with secret in projected volume
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersProjectedSecretName, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap creation: %v", err)
	}

	// Creating StatefulSet with env var source as configmap
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersConfigmapWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with configmap configmap as env var source creation: %v", err)
	}

	// Creating StatefulSet with env var source as secret
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, ersSecretWithEnvName, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in StatefulSet with secret configmap as env var source creation: %v", err)
	}

	// Creating Deployment with pod annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, ersConfigmapWithPodAnnotations, ersNamespace, false)
	if err != nil {
		logrus.Errorf("Error in Deployment with pod annotations: %v", err)
	}

	// Creating Deployment with both annotations
	_, err = testutil.CreateDeploymentWithPodAnnotations(clients.KubernetesClient, ersConfigmapWithBothAnnotations, ersNamespace, true)
	if err != nil {
		logrus.Errorf("Error in Deployment with both annotations: %v", err)
	}
}

func teardownErs() {
	// Deleting Deployment with configmap
	deploymentError := testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap in projected volume mounted in init  container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersProjectedSecretName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with secret in projected volume mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersProjectedSecretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Deployment with configmap as env var source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as env var source %v", deploymentError)
	}

	// Deleting Deployment with secret
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithEnvName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as env var source %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithInitContainer)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with secret mounted as env in init container
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithInitEnv)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret mounted as env in init container %v", deploymentError)
	}

	// Deleting Deployment with configmap as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with secret as envFrom source
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithEnvFromName)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret as envFrom source %v", deploymentError)
	}

	// Deleting Deployment with pod annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithPodAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with pod annotations %v", deploymentError)
	}

	// Deleting Deployment with both annotations
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithBothAnnotations)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with both annotations %v", deploymentError)
	}

	// Deleting Deployment with search annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapAnnotated)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with search annotation %v", deploymentError)
	}

	// Deleting Deployment with secret and secret auto annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithSecretAutoAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret auto annotation %v", deploymentError)
	}

	// Deleting Deployment with configmap and configmap auto annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapAutoAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap auto annotation %v", deploymentError)
	}

	// Deleting Deployment with secret and secret exclude annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersSecretWithSecretExcludeAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with secret exclude annotation %v", deploymentError)
	}

	// Deleting Deployment with configmap and configmap exclude annotation
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapExcludeAnnotation)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap exclude annotation %v", deploymentError)
	}

	// Deleting DaemonSet with configmap
	daemonSetError := testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersConfigmapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersSecretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting DaemonSet with configmap in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap %v", daemonSetError)
	}

	// Deleting Deployment with secret in projected volume
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersProjectedSecretName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret %v", daemonSetError)
	}

	// Deleting Deployment with configmap as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with configmap as env var source %v", daemonSetError)
	}

	// Deleting Deployment with secret as env var source
	daemonSetError = testutil.DeleteDaemonSet(clients.KubernetesClient, ersNamespace, ersSecretWithEnvName)
	if daemonSetError != nil {
		logrus.Errorf("Error while deleting daemonSet with secret as env var source %v", daemonSetError)
	}

	// Deleting StatefulSet with configmap
	statefulSetError := testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersConfigmapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersSecretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap %v", statefulSetError)
	}

	// Deleting Deployment with secret in projected volume
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersProjectedSecretName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret %v", statefulSetError)
	}

	// Deleting StatefulSet with configmap as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with configmap as env var source %v", statefulSetError)
	}

	// Deleting Deployment with secret as env var source
	statefulSetError = testutil.DeleteStatefulSet(clients.KubernetesClient, ersNamespace, ersSecretWithEnvName)
	if statefulSetError != nil {
		logrus.Errorf("Error while deleting statefulSet with secret as env var source %v", statefulSetError)
	}

	// Deleting Deployment for testing pausing deployments
	deploymentError = testutil.DeleteDeployment(clients.KubernetesClient, ersNamespace, ersConfigmapWithPausedDeployment)
	if deploymentError != nil {
		logrus.Errorf("Error while deleting deployment with configmap %v", deploymentError)
	}

	// Deleting Configmap
	err := testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Secret used in projected volume
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersProjectedSecretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting configmap used in projected volume in init containers
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersProjectedConfigMapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	// Deleting Configmap used projected volume in init containers
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersProjectedSecretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithEnvName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used in init container
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used in init container %v", err)
	}

	// Deleting Secret used in init container
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithInitContainer)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used in init container %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithEnvFromName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source %v", err)
	}

	// Deleting Configmap used as env var source
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used as env var source in init container %v", err)
	}

	// Deleting Secret used as env var source
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithInitEnv)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used as env var source in init container %v", err)
	}

	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithPodAnnotations)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with pod annotations: %v", err)
	}

	// Deleting Secret used with secret auto annotation
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithSecretAutoAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used with secret auto annotation: %v", err)
	}

	// Deleting ConfigMap used with configmap auto annotation
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapAutoAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with configmap auto annotation: %v", err)
	}

	// Deleting Secret used with secret exclude annotation
	err = testutil.DeleteSecret(clients.KubernetesClient, ersNamespace, ersSecretWithSecretExcludeAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the secret used with secret exclude annotation: %v", err)
	}

	// Deleting ConfigMap used with configmap exclude annotation
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithConfigMapExcludeAnnotation)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap used with configmap exclude annotation: %v", err)
	}

	// Deleting ConfigMap for testins pausing deployments
	err = testutil.DeleteConfigMap(clients.KubernetesClient, ersNamespace, ersConfigmapWithPausedDeployment)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap: %v", err)
	}

	// Deleting namespace
	testutil.DeleteNamespace(ersNamespace, clients.KubernetesClient)

}

func getConfigWithAnnotations(resourceType string, name string, shaData string, annotation string, typedAutoAnnotation string) util.Config {
	ns := ersNamespace
	if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
		ns = arsNamespace
	}

	return util.Config{
		Namespace:           ns,
		ResourceName:        name,
		SHAValue:            shaData,
		Annotation:          annotation,
		TypedAutoAnnotation: typedAutoAnnotation,
		Type:                resourceType,
	}
}

func getCollectors() metrics.Collectors {
	return metrics.NewCollectors()
}

var labelSucceeded = prometheus.Labels{"success": "true"}
var labelFailed = prometheus.Labels{"success": "false"}

func testRollingUpgradeInvokeDeleteStrategyArs(t *testing.T, clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, envVarPostfix string) {
	err := PerformAction(clients, config, upgradeFuncs, collectors, nil, invokeDeleteStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", upgradeFuncs.ResourceType, envVarPostfix)
	}

	config.SHAValue = testutil.GetSHAfromEmptyData()
	removed := testutil.VerifyResourceAnnotationUpdate(clients, config, upgradeFuncs)
	if !removed {
		t.Errorf("%s was not updated", upgradeFuncs.ResourceType)
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 2 {
		t.Errorf("Counter was not increased")
	}
}

func testRollingUpgradeWithPatchAndInvokeDeleteStrategyArs(t *testing.T, clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, envVarPostfix string) {
	err := PerformAction(clients, config, upgradeFuncs, collectors, nil, invokeDeleteStrategy)
	upgradeFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		return nil
	}
	upgradeFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", upgradeFuncs.ResourceType, envVarPostfix)
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	itemCalled := 0
	itemsCalled := 0

	deploymentFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetDeploymentItem(client, namespace, name)
	}
	deploymentFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetDeploymentItems(client, namespace)
	}

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	assert.Equal(t, 0, itemCalled, "ItemFunc should not be called")
	assert.Equal(t, 2, itemsCalled, "ItemsFunc should be called twice")

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithPatchAndRetryUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	assert.True(t, deploymentFuncs.SupportsPatch)
	assert.NotEmpty(t, deploymentFuncs.PatchTemplatesFunc().AnnotationTemplate)

	itemCalled := 0
	itemsCalled := 0

	deploymentFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetDeploymentItem(client, namespace, name)
	}
	deploymentFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetDeploymentItems(client, namespace)
	}

	patchCalled := 0
	deploymentFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"metadata":{"annotations":{"reloader.stakater.com/last-reloaded-from":`)
		assert.Contains(t, string(bytes), `\"hash\":\"3c9a892aeaedc759abc3df9884a37b8be5680382\"`)
		return nil
	}

	deploymentFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()
	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	assert.Equal(t, 1, itemCalled, "ItemFunc should be called once")
	assert.Equal(t, 1, itemsCalled, "ItemsFunc should be called once")
	assert.Equal(t, 2, patchCalled, "PatchFunc should be called twice")

	deploymentFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapWithoutReloadAnnotationAndWithoutAutoReloadAllNoTriggersUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigMapWithNonAnnotatedDeployment, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigMapWithNonAnnotatedDeployment, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) > 0 {
		t.Errorf("Counter by namespace was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapWithoutReloadAnnotationButWithAutoReloadAllUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	options.AutoReloadAll = true
	defer func() { options.AutoReloadAll = false }()
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigMapWithNonAnnotatedDeployment, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigMapWithNonAnnotatedDeployment, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsProjectedConfigMapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "true"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNoTriggersUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "false"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) > 0 {
		t.Errorf("Counter by namespace was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNotMappedUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	deployment, err := testutil.CreateDeploymentWithEnvVarSourceAndAnnotations(
		clients.KubernetesClient,
		arsConfigmapAnnotated+"-different",
		arsNamespace,
		map[string]string{"reloader.stakater.com/search": "true"},
	)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer func() {
		_ = clients.KubernetesClient.AppsV1().Deployments(arsNamespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	}()
	// defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "false"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) > 0 {
		t.Errorf("Counter by namespace was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapInInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectVolumeInInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsProjectedConfigMapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedConfigMapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithEnvName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithEnvName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarInInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithInitEnv, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithInitEnv, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarFromUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithEnvFromName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithEnvFromName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsProjectedSecretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretinInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolumeinInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsProjectedSecretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedSecretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithEnvName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithEnvName, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarFromUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithEnvFromName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithEnvFromName, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}
	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarInInitContainerUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithInitEnv, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithInitEnv, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretExcludeAnnotationUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithExcludeSecretAnnotation, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithExcludeSecretAnnotation, shaData, "", options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment did not update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment which had to be exluded was updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAutoAnnotationUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretWithSecretAutoAnnotation, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretWithSecretAutoAnnotation, shaData, "", options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithExcludeConfigMapAnnotationUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithExcludeConfigMapAnnotation, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithExcludeConfigMapAnnotation, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with exclude ConfigMap")
	}

	logrus.Infof("Verifying deployment did update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment which had to be excluded was updated")
	}
}

func TestRollingUpgradeForDeploymentWithConfigMapAutoAnnotationUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithConfigMapAutoAnnotation, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithConfigMapAutoAnnotation, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with ConfigMap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	itemCalled := 0
	itemsCalled := 0

	daemonSetFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetDaemonSetItem(client, namespace, name)
	}
	daemonSetFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetDaemonSetItems(client, namespace)
	}

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	assert.Equal(t, 0, itemCalled, "ItemFunc should not be called")
	assert.Equal(t, 2, itemsCalled, "ItemsFunc should be called twice")

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithPatchAndRetryUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()

	itemCalled := 0
	itemsCalled := 0

	daemonSetFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetDaemonSetItem(client, namespace, name)
	}
	daemonSetFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetDaemonSetItems(client, namespace)
	}

	assert.True(t, daemonSetFuncs.SupportsPatch)
	assert.NotEmpty(t, daemonSetFuncs.PatchTemplatesFunc().AnnotationTemplate)

	patchCalled := 0
	daemonSetFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"metadata":{"annotations":{"reloader.stakater.com/last-reloaded-from":`)
		assert.Contains(t, string(bytes), `\"hash\":\"314a2269170750a974d79f02b5b9ee517de7f280\"`)
		return nil
	}

	daemonSetFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	assert.Equal(t, 1, itemCalled, "ItemFunc should be called once")
	assert.Equal(t, 1, itemsCalled, "ItemsFunc should be called once")
	assert.Equal(t, 2, patchCalled, "PatchFunc should be called twice")

	daemonSetFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsProjectedConfigMapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapAsEnvVarUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithEnvName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithEnvName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap used as env var")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithSecretUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithSecretInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsProjectedSecretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithConfigmapUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	itemCalled := 0
	itemsCalled := 0

	statefulSetFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetStatefulSetItem(client, namespace, name)
	}
	statefulSetFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetStatefulSetItems(client, namespace)
	}

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	assert.Equal(t, 0, itemCalled, "ItemFunc should not be called")
	assert.Equal(t, 2, itemsCalled, "ItemsFunc should be called twice")

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithPatchAndRetryUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()

	itemCalled := 0
	itemsCalled := 0

	statefulSetFuncs.ItemFunc = func(client kube.Clients, namespace string, name string) (runtime.Object, error) {
		itemCalled++
		return callbacks.GetStatefulSetItem(client, namespace, name)
	}
	statefulSetFuncs.ItemsFunc = func(client kube.Clients, namespace string) []runtime.Object {
		itemsCalled++
		return callbacks.GetStatefulSetItems(client, namespace)
	}

	assert.True(t, statefulSetFuncs.SupportsPatch)
	assert.NotEmpty(t, statefulSetFuncs.PatchTemplatesFunc().AnnotationTemplate)

	patchCalled := 0
	statefulSetFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"metadata":{"annotations":{"reloader.stakater.com/last-reloaded-from":`)
		assert.Contains(t, string(bytes), `\"hash\":\"f821414d40d8815fb330763f74a4ff7ab651d4fa\"`)
		return nil
	}

	statefulSetFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	assert.Equal(t, 1, itemCalled, "ItemFunc should be called once")
	assert.Equal(t, 1, itemsCalled, "ItemsFunc should be called once")
	assert.Equal(t, 2, patchCalled, "PatchFunc should be called twice")

	statefulSetFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyArs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithConfigmapInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsProjectedConfigMapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithSecretUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsSecretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(envVarPostfix, arsSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithSecretInProjectedVolumeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, arsNamespace, arsProjectedSecretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(envVarPostfix, arsProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyArs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithPodAnnotationsUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithPodAnnotations, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithPodAnnotations, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with pod annotations")
	}

	logrus.Infof("Verifying deployment update")
	items := deploymentFuncs.ItemsFunc(clients, config.Namespace)
	var foundPod, foundBoth bool
	for _, i := range items {
		accessor, err := meta.Accessor(i)
		if err != nil {
			t.Errorf("Error getting accessor for item: %v", err)
		}
		name := accessor.GetName()
		if name == arsConfigmapWithPodAnnotations {
			annotations := deploymentFuncs.PodAnnotationsFunc(i)
			updated := testutil.GetResourceSHAFromAnnotation(annotations)
			if updated != config.SHAValue {
				t.Errorf("Deployment was not updated")
			}
			foundPod = true
		}
		if name == arsConfigmapWithBothAnnotations {
			annotations := deploymentFuncs.PodAnnotationsFunc(i)
			updated := testutil.GetResourceSHAFromAnnotation(annotations)
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

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}
}

func TestFailedRollingUpgradeUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapName, "fail.stakater.com")
	config := getConfigWithAnnotations(constants.ConfigmapEnvVarPostfix, arsConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	deploymentFuncs.UpdateFunc = func(_ kube.Clients, _ string, _ runtime.Object) error {
		return fmt.Errorf("error")
	}
	deploymentFuncs.PatchFunc = func(kube.Clients, string, runtime.Object, patchtypes.PatchType, []byte) error {
		return fmt.Errorf("error")
	}
	collectors := getCollectors()

	_ = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelFailed)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "false", "namespace": arsNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}
}

func TestIgnoreAnnotationNoReloadUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, arsNamespace, arsConfigmapWithIgnoreAnnotation, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, arsConfigmapWithIgnoreAnnotation, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/ignore": "true"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap and ignore annotation using ARS")
	}

	// Ensure deployment is NOT updated
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated but should not have been")
	}

	// Ensure counters remain zero
	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 0 {
		t.Errorf("Reload counter should not have increased")
	}
	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": arsNamespace})) != 0 {
		t.Errorf("Reload counter by namespace should not have increased")
	}
}
func TestIgnoreAnnotationNoReloadUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithIgnoreAnnotation, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithIgnoreAnnotation, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/ignore": "true"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap and ignore annotation using ERS")
	}

	// Ensure deployment is NOT updated
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated but should not have been (ERS)")
	}

	// Ensure counters remain zero
	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 0 {
		t.Errorf("Reload counter should not have increased (ERS)")
	}
	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 0 {
		t.Errorf("Reload counter by namespace should not have increased (ERS)")
	}
}

func testRollingUpgradeInvokeDeleteStrategyErs(t *testing.T, clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, envVarPostfix string) {
	err := PerformAction(clients, config, upgradeFuncs, collectors, nil, invokeDeleteStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", upgradeFuncs.ResourceType, envVarPostfix)
	}

	removed := testutil.VerifyResourceEnvVarRemoved(clients, config, envVarPostfix, upgradeFuncs)
	if !removed {
		t.Errorf("%s was not updated", upgradeFuncs.ResourceType)
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 2 {
		t.Errorf("Counter was not increased")
	}
}

func testRollingUpgradeWithPatchAndInvokeDeleteStrategyErs(t *testing.T, clients kube.Clients, config util.Config, upgradeFuncs callbacks.RollingUpgradeFuncs, collectors metrics.Collectors, envVarPostfix string) {
	assert.NotEmpty(t, upgradeFuncs.PatchTemplatesFunc().DeleteEnvVarTemplate)

	err := PerformAction(clients, config, upgradeFuncs, collectors, nil, invokeDeleteStrategy)
	upgradeFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		assert.Equal(t, patchtypes.JSONPatchType, patchType)
		assert.NotEmpty(t, bytes)
		return nil
	}
	upgradeFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", upgradeFuncs.ResourceType, envVarPostfix)
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithPatchAndRetryUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	assert.True(t, deploymentFuncs.SupportsPatch)
	assert.NotEmpty(t, deploymentFuncs.PatchTemplatesFunc().EnvVarTemplate)

	patchCalled := 0
	deploymentFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"spec":{"containers":[{"name":`)
		assert.Contains(t, string(bytes), `"value":"3c9a892aeaedc759abc3df9884a37b8be5680382"`)
		return nil
	}

	deploymentFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	assert.Equal(t, 2, patchCalled)

	deploymentFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersProjectedConfigMapName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "true"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNoTriggersUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "false"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	time.Sleep(5 * time.Second)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) > 0 {
		t.Errorf("Counter by namespace was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapViaSearchAnnotationNotMappedUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	deployment, err := testutil.CreateDeploymentWithEnvVarSourceAndAnnotations(
		clients.KubernetesClient,
		ersConfigmapAnnotated+"-different",
		ersNamespace,
		map[string]string{"reloader.stakater.com/search": "true"},
	)
	if err != nil {
		t.Errorf("Failed to create deployment with search annotation.")
	}
	defer func() {
		_ = clients.KubernetesClient.AppsV1().Deployments(ersNamespace).Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
	}()
	// defer clients.KubernetesClient.AppsV1().Deployments(namespace).Delete(deployment.Name, &v1.DeleteOptions{})

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapAnnotated, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapAnnotated, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	config.ResourceAnnotations = map[string]string{"reloader.stakater.com/match": "false"}
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment was updated unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) > 0 {
		t.Errorf("Counter was increased unexpectedly")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) > 0 {
		t.Errorf("Counter by namespace was increased unexpectedly")
	}
}

func TestRollingUpgradeForDeploymentWithConfigmapInInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for %s with %s", deploymentFuncs.ResourceType, envVarPostfix)
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapInProjectVolumeInInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersProjectedConfigMapWithInitContainer, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedConfigMapWithInitContainer, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithEnvName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithEnvName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarInInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithInitEnv, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithInitEnv, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigmapAsEnvVarFromUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithEnvFromName, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithEnvFromName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Configmap used as env var")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersProjectedSecretName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretinInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretInProjectedVolumeinInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersProjectedSecretWithInitContainer, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedSecretWithInitContainer, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret in projected volume")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithEnvName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithEnvName, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarFromUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithEnvFromName, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithEnvFromName, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretAsEnvVarInInitContainerUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithInitEnv, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithInitEnv, shaData, options.ReloaderAutoAnnotation, options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithSecretExcludeAnnotationUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithSecretExcludeAnnotation, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithSecretExcludeAnnotation, shaData, "", options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with exclude Secret")
	}

	logrus.Infof("Verifying deployment did not update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment that had to be excluded was updated")
	}
}

func TestRollingUpgradeForDeploymentWithSecretAutoAnnotationUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretWithSecretAutoAnnotation, "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretWithSecretAutoAnnotation, shaData, "", options.SecretReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with Secret")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithConfigMapExcludeAnnotationUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithConfigMapExcludeAnnotation, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithConfigMapExcludeAnnotation, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with exclude ConfigMap")
	}

	logrus.Infof("Verifying deployment did not update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment which had to be excluded was updated")
	}
}

func TestRollingUpgradeForDeploymentWithConfigMapAutoAnnotationUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithConfigMapAutoAnnotation, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithConfigMapAutoAnnotation, shaData, "", options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with ConfigMap")
	}

	logrus.Infof("Verifying deployment update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, deploymentFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithPatchAndRetryUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()

	assert.True(t, daemonSetFuncs.SupportsPatch)
	assert.NotEmpty(t, daemonSetFuncs.PatchTemplatesFunc().EnvVarTemplate)

	patchCalled := 0
	daemonSetFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"spec":{"containers":[{"name":`)
		assert.Contains(t, string(bytes), `"value":"314a2269170750a974d79f02b5b9ee517de7f280"`)
		return nil
	}

	daemonSetFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap")
	}

	assert.Equal(t, 2, patchCalled)

	daemonSetFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersProjectedConfigMapName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithConfigmapAsEnvVarUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithEnvName, "www.facebook.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithEnvName, shaData, options.ReloaderAutoAnnotation, options.ConfigmapReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with configmap used as env var")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithSecretUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDaemonSetWithSecretInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersProjectedSecretName, "d3d3LmZhY2Vib29rLmNvbQ==")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	daemonSetFuncs := GetDaemonSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, daemonSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for DaemonSet with secret in projected volume")
	}

	logrus.Infof("Verifying daemonSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, daemonSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithConfigmapUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithPatchAndRetryUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()

	assert.True(t, statefulSetFuncs.SupportsPatch)
	assert.NotEmpty(t, statefulSetFuncs.PatchTemplatesFunc().EnvVarTemplate)

	patchCalled := 0
	statefulSetFuncs.PatchFunc = func(client kube.Clients, namespace string, resource runtime.Object, patchType patchtypes.PatchType, bytes []byte) error {
		patchCalled++
		if patchCalled < 2 {
			return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonConflict}} // simulate conflict
		}
		assert.Equal(t, patchtypes.StrategicMergePatchType, patchType)
		assert.NotEmpty(t, bytes)
		assert.Contains(t, string(bytes), `{"spec":{"template":{"spec":{"containers":[{"name":`)
		assert.Contains(t, string(bytes), `"value":"f821414d40d8815fb330763f74a4ff7ab651d4fa"`)
		return nil
	}

	statefulSetFuncs.UpdateFunc = func(kube.Clients, string, runtime.Object) error {
		t.Errorf("Update should not be called")
		return nil
	}

	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap")
	}

	assert.Equal(t, 2, patchCalled)

	statefulSetFuncs = GetDeploymentRollingUpgradeFuncs()
	testRollingUpgradeWithPatchAndInvokeDeleteStrategyErs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithConfigmapInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersProjectedConfigMapName, "www.twitter.com")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedConfigMapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with configmap in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithSecretUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersSecretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(envVarPostfix, ersSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForStatefulSetWithSecretInProjectedVolumeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.SecretEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, ersNamespace, ersProjectedSecretName, "d3d3LnR3aXR0ZXIuY29t")
	config := getConfigWithAnnotations(envVarPostfix, ersProjectedSecretName, shaData, options.SecretUpdateOnChangeAnnotation, options.SecretReloaderAutoAnnotation)
	statefulSetFuncs := GetStatefulSetRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, statefulSetFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for StatefulSet with secret in projected volume")
	}

	logrus.Infof("Verifying statefulSet update")
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, envVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	testRollingUpgradeInvokeDeleteStrategyErs(t, clients, config, statefulSetFuncs, collectors, envVarPostfix)
}

func TestRollingUpgradeForDeploymentWithPodAnnotationsUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapWithPodAnnotations, "www.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapWithPodAnnotations, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	err := PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)
	time.Sleep(5 * time.Second)
	if err != nil {
		t.Errorf("Rolling upgrade failed for Deployment with pod annotations")
	}

	logrus.Infof("Verifying deployment update")
	envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(config.ResourceName) + "_" + envVarPostfix
	items := deploymentFuncs.ItemsFunc(clients, config.Namespace)
	var foundPod, foundBoth bool
	for _, i := range items {
		accessor, err := meta.Accessor(i)
		if err != nil {
			t.Errorf("Error getting accessor for item: %v", err)
		}
		name := accessor.GetName()
		if name == ersConfigmapWithPodAnnotations {
			containers := deploymentFuncs.ContainersFunc(i)
			updated := testutil.GetResourceSHAFromEnvVar(containers, envName)
			if updated != config.SHAValue {
				t.Errorf("Deployment was not updated")
			}
			foundPod = true
		}
		if name == ersConfigmapWithBothAnnotations {
			containers := deploymentFuncs.ContainersFunc(i)
			updated := testutil.GetResourceSHAFromEnvVar(containers, envName)
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

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}
}

func TestFailedRollingUpgradeUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, ersNamespace, ersConfigmapName, "fail.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, ersConfigmapName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	deploymentFuncs.UpdateFunc = func(_ kube.Clients, _ string, _ runtime.Object) error {
		return fmt.Errorf("error")
	}
	deploymentFuncs.PatchFunc = func(kube.Clients, string, runtime.Object, patchtypes.PatchType, []byte) error {
		return fmt.Errorf("error")
	}
	collectors := getCollectors()

	_ = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelFailed)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "false", "namespace": ersNamespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}
}

func TestPausingDeploymentUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy
	testPausingDeployment(t, options.ReloadStrategy, ersConfigmapWithPausedDeployment, ersNamespace)
}

func TestPausingDeploymentUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy
	testPausingDeployment(t, options.ReloadStrategy, arsConfigmapWithPausedDeployment, arsNamespace)
}

func testPausingDeployment(t *testing.T, reloadStrategy string, testName string, namespace string) {
	options.ReloadStrategy = reloadStrategy
	envVarPostfix := constants.ConfigmapEnvVarPostfix

	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, testName, "pause.stakater.com")
	config := getConfigWithAnnotations(envVarPostfix, testName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)
	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()
	collectors := getCollectors()

	_ = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 1 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": namespace})) != 1 {
		t.Errorf("Counter by namespace was not increased")
	}

	logrus.Infof("Verifying deployment has been paused")
	items := deploymentFuncs.ItemsFunc(clients, config.Namespace)
	deploymentPaused, err := isDeploymentPaused(items, testName)
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	if !deploymentPaused {
		t.Errorf("Deployment has not been paused")
	}

	shaData = testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, testName, "pause-changed.stakater.com")
	config = getConfigWithAnnotations(envVarPostfix, testName, shaData, options.ConfigmapUpdateOnChangeAnnotation, options.ConfigmapReloaderAutoAnnotation)

	_ = PerformAction(clients, config, deploymentFuncs, collectors, nil, invokeReloadStrategy)

	if promtestutil.ToFloat64(collectors.Reloaded.With(labelSucceeded)) != 2 {
		t.Errorf("Counter was not increased")
	}

	if promtestutil.ToFloat64(collectors.ReloadedByNamespace.With(prometheus.Labels{"success": "true", "namespace": namespace})) != 2 {
		t.Errorf("Counter by namespace was not increased")
	}

	logrus.Infof("Verifying deployment is still paused")
	items = deploymentFuncs.ItemsFunc(clients, config.Namespace)
	deploymentPaused, err = isDeploymentPaused(items, testName)
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	if !deploymentPaused {
		t.Errorf("Deployment should still be paused")
	}

	logrus.Infof("Verifying deployment has been resumed after pause interval")
	time.Sleep(11 * time.Second)
	items = deploymentFuncs.ItemsFunc(clients, config.Namespace)
	deploymentPaused, err = isDeploymentPaused(items, testName)
	if err != nil {
		t.Errorf("%s", err.Error())
	}
	if deploymentPaused {
		t.Errorf("Deployment should have been resumed after pause interval")
	}
}

func isDeploymentPaused(deployments []runtime.Object, deploymentName string) (bool, error) {
	deployment, err := FindDeploymentByName(deployments, deploymentName)
	if err != nil {
		return false, err
	}
	return IsPaused(deployment), nil
}

// Simple helper function for test cases
func FindDeploymentByName(deployments []runtime.Object, deploymentName string) (*apps.Deployment, error) {
	for _, deployment := range deployments {
		accessor, err := meta.Accessor(deployment)
		if err != nil {
			return nil, fmt.Errorf("error getting accessor for item: %v", err)
		}
		if accessor.GetName() == deploymentName {
			deploymentObj, ok := deployment.(*apps.Deployment)
			if !ok {
				return nil, fmt.Errorf("failed to cast to Deployment")
			}
			return deploymentObj, nil
		}
	}
	return nil, fmt.Errorf("deployment '%s' not found", deploymentName)
}

// Checks if a deployment is currently paused
func IsPaused(deployment *apps.Deployment) bool {
	return deployment.Spec.Paused
}
