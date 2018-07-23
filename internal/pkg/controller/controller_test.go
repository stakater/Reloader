package controller

import (
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/common"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	client              = getClient()
	namespace           = "test-reloader"
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix    = "testsecret-reloader"
	data                = "dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	newData             = "dGVzdE5ld1NlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI="
	updatedData         = "dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy"
)

func TestMain(m *testing.M) {

	logrus.Infof("Creating namespace %s", namespace)
	testutil.CreateNamespace(namespace, client)

	logrus.Infof("Running Testcases")
	retCode := m.Run()

	logrus.Infof("Deleting namespace %q.\n", namespace)
	testutil.DeleteNamespace(namespace, client)

	os.Exit(retCode)
}

func getClient() *kubernetes.Clientset {
	newClient, err := kube.GetClient()
	if err != nil {
		logrus.Fatalf("Unable to create Kubernetes client error = %v", err)
	}
	return newClient
}

// Perform rolling upgrade on deployment and create env var upon updating the configmap
func TestControllerUpdatingConfigmapShouldCreateEnvInDeployment(t *testing.T) {
	// Creating Controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating Controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying deployment update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.google.com")
	updated := testutil.VerifyDeploymentUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyDeploymentUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyDeploymentUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyDeploymentUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating Controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying DaemonSet update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying DaemonSet update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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

	// Updating Secret
	err = testutil.UpdateSecret(secretClient, namespace, secretName, "", updatedData)
	if err != nil {
		t.Errorf("Error while updating secret %v", err)
	}

	// Verifying Upgrade
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.SecretResourceType, namespace, secretName, updatedData)
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyDaemonSetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating Controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.stakater.com")
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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
	// Creating controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Updating configmap for second time
	updateErr = testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "", "aurorasolutions.io")
	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying env var has been updated")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "aurorasolutions.io")
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
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

// Do not Perform rolling upgrade on StatefulSet and create env var upon updating the labels configmap
func TestControllerUpdatingConfigmapLabelsShouldNotCreateorUpdateEnvInStatefulSet(t *testing.T) {
	// Creating Controller
	logrus.Infof("Creating controller")
	controller, err := NewController(client, testutil.ConfigmapResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	logrus.Infof("Starting controller")
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating configmap
	configmapName := configmapNamePrefix + "-update-" + common.RandSeq(5)
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
	updateErr := testutil.UpdateConfigMap(configmapClient, namespace, configmapName, "test", "www.google.com")
	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		t.Errorf("Configmap was not updated")
	}

	// Verifying StatefulSet update
	logrus.Infof("Verifying env var has been created")
	shaData := testutil.ConvertResourceToSHA(testutil.ConfigmapResourceType, namespace, configmapName, "www.google.com")
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, configmapName, common.ConfigmapEnvarPostfix, shaData, common.ConfigmapUpdateOnChangeAnnotation)
	if updated {
		t.Errorf("StatefulSet should not be updated by changing label")
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
	// Creating controller
	controller, err := NewController(client, testutil.SecretResourceType, namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(5 * time.Second)

	// Creating secret
	secretName := secretNamePrefix + "-update-" + common.RandSeq(5)
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
	updated := testutil.VerifyStatefulSetUpdate(client, namespace, secretName, common.SecretEnvarPostfix, shaData, common.SecretUpdateOnChangeAnnotation)
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
