package controller

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/callbacks"
	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/kube"
)

var (
	client              = testutil.GetClient()
	namespace           = "test-reloader-" + testutil.RandSeq(5)
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix    = "testsecret-reloader"
	data                = "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	newData             = "dGVzdE5ld1NlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	updatedData         = "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"
)

func TestMain(m *testing.M) {

	logrus.Infof("Creating namespace %s", namespace)
	testutil.CreateNamespace(namespace, client)

	logrus.Infof("Creating controller")
	for k := range kube.ResourceMap {
		c, err := NewController(client, k, namespace)
		if err != nil {
			logrus.Fatalf("%s", err)
		}

		// Now let's start the controller
		stop := make(chan struct{})
		defer close(stop)
		go c.Run(1, stop)
	}
	time.Sleep(5 * time.Second)

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	logrus.Infof("Deleting namespace %q.\n", namespace)
	testutil.DeleteNamespace(namespace, client)

	os.Exit(retCode)
}

// Perform rolling upgrade on deployment and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDeployment(t *testing.T) {

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting deployment
	err = testutil.DeleteDeployment(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on deployment and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDeployment(t *testing.T) {
	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}

	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}

	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting deployment
	err = testutil.DeleteDeployment(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Do not Perform rolling upgrade on deployment and create env var upon updating the labels configmap
func TestControllerUpdatingConfigmapLabelsShouldNotCreateorUpdateEnvInDeployment(t *testing.T) {
	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label")
	}
	time.Sleep(5 * time.Second)

	// Deleting deployment
	err = testutil.DeleteDeployment(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on secret and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInDeployment(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting Deployment
	err = testutil.DeleteDeployment(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on deployment and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInDeployment(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if !updated {
		t.Errorf("Deployment was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting Deployment
	err = testutil.DeleteDeployment(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Do not Perform rolling upgrade on secret and create or update a env var upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateorUpdateEnvInDeployment(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating deployment
	_, err = testutil.CreateDeployment(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	deploymentFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDeploymentItems,
		ContainersFunc: callbacks.GetDeploymentContainers,
		UpdateFunc:     callbacks.UpdateDeployment,
		ResourceType:   "Deployment",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, deploymentFuncs)
	if updated {
		t.Errorf("Deployment should not be updated by changing label in secret")
	}
	//time.Sleep(5 * time.Second)

	// Deleting Deployment
	err = testutil.DeleteDeployment(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the deployment %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on DaemonSet and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDaemonSet(t *testing.T) {
	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on DaemonSet and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateDaemonSet(t *testing.T) {
	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(client, configmapName, namespace)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
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

	// Verifying DaemonSet update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	config := util.Config{
		Namespace:    namespace,
		ResourceName: configmapName,
		SHAValue:     shaData,
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on secret and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInDaemonSet(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on DaemonSet and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInDaemonSet(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(client, secretName, namespace)
	if err != nil {
		t.Errorf("Error  in DaemonSet creation: %v", err)
	}

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", newData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}
	time.Sleep(5 * time.Second)

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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if !updated {
		t.Errorf("DaemonSet was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Do not Perform rolling upgrade on secret and create or update a env var upon updating the label in secret
func TestControllerUpdatingSecretLabelsShouldNotCreateorUpdateEnvInDaemonSet(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating DaemonSet
	_, err = testutil.CreateDaemonSet(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	daemonSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetDaemonSetItems,
		ContainersFunc: callbacks.GetDaemonSetContainers,
		UpdateFunc:     callbacks.UpdateDaemonSet,
		ResourceType:   "DaemonSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, daemonSetFuncs)
	if updated {
		t.Errorf("DaemonSet should not be updated by changing label in secret")
	}
	//time.Sleep(5 * time.Second)

	// Deleting DaemonSet
	err = testutil.DeleteDaemonSet(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the DaemonSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on StatefulSet and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInStatefulSet(t *testing.T) {
	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		ResourceType:   "StatefulSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on StatefulSet and update env var upon updating the configmap
func TestControllerForUpdatingConfigmapShouldUpdateStatefulSet(t *testing.T) {
	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + testutil.RandSeq(5)
	configmapClient, err := testutil.CreateConfigMap(client, namespace, configmapName, "www.google.com")
	if err != nil {
		t.Errorf("Error while creating the configmap %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(client, configmapName, namespace)
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
		Annotation:   constants.ConfigmapUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		ResourceType:   "StatefulSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.ConfigmapEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	time.Sleep(5 * time.Second)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	// Deleting configmap
	err = testutil.DeleteConfigMap(client, namespace, configmapName)
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on secret and create a env var upon updating the secret
func TestControllerUpdatingSecretShouldCreateEnvInStatefulSet(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		ResourceType:   "StatefulSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}

// Perform rolling upgrade on StatefulSet and update env var upon updating the secret
func TestControllerUpdatingSecretShouldUpdateEnvInStatefulSet(t *testing.T) {
	// Creating secret
	secretName := secretNamePrefix + "-update-" + testutil.RandSeq(5)
	secretClient, err := testutil.CreateSecret(client, namespace, secretName, data)
	if err != nil {
		t.Errorf("Error  in secret creation: %v", err)
	}

	// Creating StatefulSet
	_, err = testutil.CreateStatefulSet(client, secretName, namespace)
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
		Annotation:   constants.SecretUpdateOnChangeAnnotation,
	}
	statefulSetFuncs := callbacks.RollingUpgradeFuncs{
		ItemsFunc:      callbacks.GetStatefulSetItems,
		ContainersFunc: callbacks.GetStatefulsetContainers,
		UpdateFunc:     callbacks.UpdateStatefulset,
		ResourceType:   "StatefulSet",
	}
	updated := testutil.VerifyResourceUpdate(client, config, constants.SecretEnvVarPostfix, statefulSetFuncs)
	if !updated {
		t.Errorf("StatefulSet was not updated")
	}
	//time.Sleep(5 * time.Second)

	// Deleting StatefulSet
	err = testutil.DeleteStatefulSet(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the StatefulSet %v", err)
	}

	//Deleting Secret
	err = testutil.DeleteSecret(client, namespace, secretName)
	if err != nil {
		logrus.Errorf("Error while deleting the secret %v", err)
	}
	time.Sleep(5 * time.Second)
}
