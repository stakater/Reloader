package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/constants"

	"github.com/stakater/Reloader/internal/pkg/metrics"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/handler"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	clients             = kube.GetClients()
	namespace           = "test-reloader-" + testutil.RandSeq(5)
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix    = "testsecret-reloader"
	data                = "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	newData             = "dGVzdE5ld1NlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	updatedData         = "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"
	collectors          = metrics.NewCollectors()
)

const (
	sleepDuration = 3 * time.Second
)

func TestMain(m *testing.M) {

	testutil.CreateNamespace(namespace, clients.KubernetesClient)

	logrus.Infof("Creating controller")
	for k := range kube.ResourceMap {
		c, err := NewController(clients.KubernetesClient, k, namespace, []string{}, map[string]string{}, collectors)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)
		go c.Run(1, stop)
	}
	time.Sleep(sleepDuration)

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	testutil.DeleteNamespace(namespace, clients.KubernetesClient)

	os.Exit(retCode)
}

// Perform rolling upgrade on deploymentConfig and create pod annotation var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreatePodAnnotationInDeploymentConfig(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Don't run test on non-openshift environment
	if !kube.IsOpenshift {
		return
	}

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeploymentConfig(clients.OpenshiftAppsClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error in deploymentConfig creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentConfigFuncs := handler.GetDeploymentConfigRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentConfigFuncs)
	if !updated {
		t.Errorf("DeploymentConfig was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeploymentConfig(clients.OpenshiftAppsClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deploymentConfig %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create pod annotation var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create pod annotation var upon updating the configmap
func TestControllerUpdatingConfigmapShouldAutoCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, false)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create pod annotation var upon creating the configmap
func TestControllerCreatingConfigmapShouldCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// TODO: Fix this test case
	t.Skip("Skipping TestControllerCreatingConfigmapShouldCreatePodAnnotationInDeployment test case")

	// Creating configmap
	configmapName := configmapNamePrefix + "-create-" + testutil.RandSeq(5)
	_, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Deleting configmap for first time
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	time.Sleep(sleepDuration)

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.stakater.com")
	if err != nil {
		t.Errorf("Error while creating the configmap second time %v", err)
	}

	time.Sleep(sleepDuration)

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and update pod annotation var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDeploymentUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}

	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()

	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on deployment and create pod annotation var upon updating the labels configmap
func TestControllerUpdatingConfigmapLabelsShouldNotCreateOrCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "test", "www.google.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.google.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create pod annotation  var upon creating the secret
func TestControllerCreatingSecretShouldCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// TODO: Fix this test case
	t.Skip("Skipping TestControllerCreatingConfigmapShouldCreatePodAnnotationInDeployment test case")

	// Creating secret
	secretName := secretNamePrefix + "-create-" + testutil.RandSeq(5)
	_, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)

	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, newData)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	time.Sleep(sleepDuration)

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	time.Sleep(sleepDuration)
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create pod annotation  var upon updating the secret
func TestControllerUpdatingSecretShouldCreatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and update pod annotation var upon updating the secret
func TestControllerUpdatingSecretShouldUpdatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on pod and create or update a pod annotation upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateOrUpdatePodAnnotationInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	err = testutil.UpdateSecret(secretClient, namespace, secretName, "test", data)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, data)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label in secret")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and create pod annotation var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreatePodAnnotationInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying DaemonSet update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and update pod annotation var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDaemonSetUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	time.Sleep(sleepDuration)

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	time.Sleep(sleepDuration)

	// Verifying DaemonSet update
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create pod annotation  var upon updating the secret
func TestControllerUpdatingSecretShouldCreatePodAnnotationInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and update pod annotation var upon updating the secret
func TestControllerUpdatingSecretShouldUpdatePodAnnotationInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}
	time.Sleep(sleepDuration)

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on pod and create or update a pod annotation upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateOrUpdatePodAnnotationInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	err = testutil.UpdateSecret(secretClient, namespace, secretName, "test", data)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, data)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, daemonSetFuncs)
	if updated {
		t.Errorf("DaemonSet should not be updated by changing label in secret")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and create pod annotation var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreatePodAnnotationInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and update pod annotation var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateStatefulSetUsingArs(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create pod annotation  var upon updating the secret
func TestControllerUpdatingSecretShouldCreatePodAnnotationInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deploymentConfig and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDeploymentConfig(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Don't run test on non-openshift environment
	if !kube.IsOpenshift {
		return
	}

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeploymentConfig(clients.OpenshiftAppsClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error in deploymentConfig creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentConfigFuncs := handler.GetDeploymentConfigRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentConfigFuncs)
	if !updated {
		t.Errorf("DeploymentConfig was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeploymentConfig(clients.OpenshiftAppsClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deploymentConfig %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldAutoCreateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, false)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and create env var upon creating the configmap
func TestControllerCreatingConfigmapShouldCreateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// TODO: Fix this test case
	t.Skip("Skipping TestControllerCreatingConfigmapShouldCreateEnvInDeployment test case")

	// Creating configmap
	configmapName := configmapNamePrefix + "-create-" + testutil.RandSeq(5)
	_, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Deleting configmap for first time
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}

	time.Sleep(sleepDuration)

	_, err = testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.stakater.com")
	if err != nil {
		t.Errorf("Error while creating the configmap second time %v", err)
	}

	time.Sleep(sleepDuration)

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDeploymentUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}

	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()

	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on deployment and create env var upon updating the labels configmap
func TestControllerUpdatingConfigmapLabelsShouldNotCreateOrUpdateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "test", "www.google.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.google.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label")
	}
	time.Sleep(sleepDuration)

	// Deleting deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create a env var upon creating the secret
func TestControllerCreatingSecretShouldCreateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// TODO: Fix this test case
	t.Skip("Skipping TestControllerCreatingConfigmapShouldCreateEnvInDeployment test case")

	// Creating secret
	secretName := secretNamePrefix + "-create-" + testutil.RandSeq(5)
	_, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)

	_, err = testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, newData)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	time.Sleep(sleepDuration)

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	time.Sleep(sleepDuration)
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on deployment and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on pod and create or update a env var upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateOrUpdateEnvInDeployment(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in deployment creation: %v", err)
	}

	err = testutil.UpdateSecret(secretClient, namespace, secretName, "test", data)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, data)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := handler.GetDeploymentRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label in secret")
	}

	// Deleting Deployment
	err = testutil.DeleteDeployment(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying DaemonSet update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDaemonSetUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	time.Sleep(sleepDuration)

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	time.Sleep(sleepDuration)

	// Verifying DaemonSet update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on DaemonSet and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}
	time.Sleep(sleepDuration)

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Do not Perform rolling upgrade on pod and create or update a env var upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateOrUpdateEnvInDaemonSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	err = testutil.UpdateSecret(secretClient, namespace, secretName, "test", data)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, data)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := handler.GetDaemonSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if updated {
		t.Errorf("DaemonSet should not be updated by changing label in secret")
	}

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateStatefulSetUsingErs(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(clients.KubernetesClient, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, configmapName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating configmap for first time
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "www.stakater.com")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   options.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(sleepDuration)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(clients.KubernetesClient, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on pod and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, newData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.EnvVarsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceEnvVarUpdate(clients, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

// Perform rolling upgrade on StatefulSet and update pod annotation var upon updating the secret
func TestControllerUpdatingSecretShouldUpdatePodAnnotationInStatefulSet(t *testing.T) {
	options.ReloadStrategy = constants.AnnotationsReloadStrategy

	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(clients.KubernetesClient, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(clients.KubernetesClient, secretName, namespace, true)
	if err != nil {
		t.Errorf("Error  in StatefulSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying pod annotation has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	config := util.Config{
		Namespace:    namespace,
		ResourceName: secretName,
		SHAValue:     shaData,
		Annotation:   options.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := handler.GetStatefulSetRollingUpgradeFuncs()
	updated := testutil.VerifyResourceAnnotationUpdate(clients, config, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(clients.KubernetesClient, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(sleepDuration)
}

func TestController_resourceInIgnoredNamespace(t *testing.T) {
	type fields struct {
		client            kubernetes.Interface
		indexer           cache.Indexer
		queue             workqueue.RateLimitingInterface
		informer          cache.Controller
		namespace         string
		ignoredNamespaces util.List
	}
	type args struct {
		raw interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "TestConfigMapResourceInIgnoredNamespaceShouldReturnTrue",
			fields: fields{
				ignoredNamespaces: util.List{
					"system",
				},
			},
			args: args{
				raw: testutil.GetConfigmap("system", "testcm", "test"),
			},
			want: true,
		},
		{
			name: "TestSecretResourceInIgnoredNamespaceShouldReturnTrue",
			fields: fields{
				ignoredNamespaces: util.List{
					"system",
				},
			},
			args: args{
				raw: testutil.GetSecret("system", "testsecret", "test"),
			},
			want: true,
		},
		{
			name: "TestConfigMapResourceInIgnoredNamespaceShouldReturnFalse",
			fields: fields{
				ignoredNamespaces: util.List{
					"system",
				},
			},
			args: args{
				raw: testutil.GetConfigmap("some-other-namespace", "testcm", "test"),
			},
			want: false,
		},
		{
			name: "TestConfigMapResourceInIgnoredNamespaceShouldReturnFalse",
			fields: fields{
				ignoredNamespaces: util.List{
					"system",
				},
			},
			args: args{
				raw: testutil.GetSecret("some-other-namespace", "testsecret", "test"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				client:            tt.fields.client,
				indexer:           tt.fields.indexer,
				queue:             tt.fields.queue,
				informer:          tt.fields.informer,
				namespace:         tt.fields.namespace,
				ignoredNamespaces: tt.fields.ignoredNamespaces,
			}
			if got := c.resourceInIgnoredNamespace(tt.args.raw); got != tt.want {
				t.Errorf("Controller.resourceInIgnoredNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_resourceInNamespaceSelector(t *testing.T) {
	type fields struct {
		indexer           cache.Indexer
		queue             workqueue.RateLimitingInterface
		informer          cache.Controller
		namespace         v1.Namespace
		namespaceSelector util.Map
	}
	type args struct {
		raw interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "TestConfigMapResourceInNamespaceSelector",
			fields: fields{
				namespaceSelector: util.Map{
					"select":  "this",
					"select2": "this2",
				},
				namespace: v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "selected-namespace",
						Labels: map[string]string{
							"select":  "this",
							"select2": "this2",
						},
					},
				},
			},
			args: args{
				raw: testutil.GetConfigmap("selected-namespace", "testcm", "test"),
			},
			want: true,
		}, {
			name: "TestConfigMapResourceNotInNamespaceSelector",
			fields: fields{
				namespaceSelector: util.Map{
					"select":  "this",
					"select2": "this2",
				},
				namespace: v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "not-selected-namespace",
						Labels: map[string]string{},
					},
				},
			},
			args: args{
				raw: testutil.GetConfigmap("not-selected-namespace", "testcm", "test"),
			},
			want: false,
		},
		{
			name: "TestSecretResourceInNamespaceSelector",
			fields: fields{
				namespaceSelector: util.Map{
					"select":  "this",
					"select2": "this2",
				},
				namespace: v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "selected-namespace",
						Labels: map[string]string{
							"select":  "this",
							"select2": "this2",
						},
					},
				},
			},
			args: args{
				raw: testutil.GetSecret("selected-namespace", "testsecret", "test"),
			},
			want: true,
		}, {
			name: "TestSecretResourceNotInNamespaceSelector",
			fields: fields{
				namespaceSelector: util.Map{
					"select":  "this",
					"select2": "this2",
				},
				namespace: v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "not-selected-namespace",
						Labels: map[string]string{},
					},
				},
			},
			args: args{
				raw: testutil.GetSecret("not-selected-namespace", "secret", "test"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			fakeClient.CoreV1().Namespaces().Create(context.Background(), &tt.fields.namespace, metav1.CreateOptions{})

			c := &Controller{
				client:            fakeClient,
				indexer:           tt.fields.indexer,
				queue:             tt.fields.queue,
				informer:          tt.fields.informer,
				namespace:         tt.fields.namespace.ObjectMeta.Name,
				namespaceSelector: tt.fields.namespaceSelector,
			}

			if got := c.resourceInNamespaceSelector(tt.args.raw); got != tt.want {
				t.Errorf("Controller.resourceInNamespaceSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}
