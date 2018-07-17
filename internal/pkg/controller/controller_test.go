package controller

import (
	"math/rand"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix    = "testsecret-reloader"
	letters             = []rune("abcdefghijklmnopqrstuvwxyz")
)

func randSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Creating a Controller to do a rolling upgrade upon updating the configmap or secret
func TestControllerForUpdatingConfigmapShouldUpdateDeployment(t *testing.T) {
	client, err := kube.GetClient()
	if err != nil {
		logrus.Errorf("Unable to create Kubernetes client error = %v", err)
		return
	}
	namespace := "test-reloader"
	createNamespace(t, namespace, client)
	defer deleteNamespace(t, namespace, client)

	controller, err := NewController(client, "configMaps", namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	configmapName := configmapNamePrefix + "-update-" + randSeq(5)
	configmapClient := client.CoreV1().ConfigMaps(namespace)
	_, err = configmapClient.Create(initConfigmap(namespace, configmapName))
	if err != nil {
		logrus.Fatalf("Fatal error  in configmap creation: %v", err)
	}
	logrus.Infof("Created Configmap %q.\n", configmapName)
	time.Sleep(10 * time.Second)
	deployment := createDeployement(configmapName, namespace, client)

	logrus.Infof("Updating Configmap %q.\n", configmapName)
	_, err = configmapClient.Get(configmapName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting configmap %v", err)
	}
	_, updateErr := configmapClient.Update(updateConfigmap(namespace, configmapName))

	// TODO: Add functionality to verify reloader functionality here

	if updateErr != nil {
		err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		if err != nil {
			logrus.Errorf("Error while deleting the configmap %v", err)
		}
		logrus.Fatalf("Fatal error  in configmap update: %v", updateErr)
	}
	time.Sleep(10 * time.Second)
	logrus.Infof("Deleting Deployment %q.\n", deployment.GetObjectMeta().GetName())
	deploymentError := controller.client.ExtensionsV1beta1().Deployments(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if deploymentError != nil {
		logrus.Fatalf("Error while deleting the configmap %v", deploymentError)
	}
	logrus.Infof("Deleting Configmap %q.\n", configmapName)
	err = controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	if err != nil {
		logrus.Errorf("Error while deleting the configmap %v", err)
	}
	time.Sleep(15 * time.Second)
}

func createDeployement(deploymentName string, namespace string, client kubernetes.Interface) *v1beta1.Deployment {
	deploymentClient := client.ExtensionsV1beta1().Deployments(namespace)
	deployment := initDeployment(namespace, deploymentName)
	deployment, err := deploymentClient.Create(deployment)
	if err != nil {
		logrus.Fatalf("Fatal error  in deployment creation: %v", err)
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
	createNamespace(t, namespace, client)
	defer deleteNamespace(t, namespace, client)

	controller, err := NewController(client, "secrets", namespace)
	if err != nil {
		logrus.Errorf("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	secretName := secretNamePrefix + "-update-" + randSeq(5)
	secretClient := client.CoreV1().Secrets(namespace)
	_, err = secretClient.Create(initSecret(namespace, secretName))
	if err != nil {
		logrus.Fatalf("Fatal error  in secret creation: %v", err)
	}
	logrus.Infof("Created Secret %q.\n", secretName)
	time.Sleep(10 * time.Second)

	logrus.Infof("Updating Secret %q.\n", secretName)
	_, err = secretClient.Get(secretName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error while getting secret %v", err)
	}
	_, updateErr := secretClient.Update(updateSecret(namespace, secretName))

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

func initConfigmap(namespace string, configmapName string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string]string{"test.url": "www.google.com"},
	}
}

func initDeployment(namespace string, deploymentName string) *v1beta1.Deployment {
	replicaset := int32(1)
	return &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deploymentName,
			Namespace:   namespace,
			Labels:      map[string]string{"firstLabel": "temp"},
			Annotations: map[string]string{"reloader.stakater.com/configmap.update-on-change": deploymentName},
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicaset,
			Strategy: v1beta1.DeploymentStrategy{
				Type: v1beta1.RollingUpdateDeploymentStrategyType,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"secondLabel": "temp"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "tutum/hello-world",
							Name:  deploymentName,
							Env: []v1.EnvVar{
								{
									Name:  "BUCKET_NAME",
									Value: "test",
								},
							},
						},
					},
				},
			},
		},
	}
}

func initSecret(namespace string, secretName string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string][]byte{"test.url": []byte("dGVzdFNlY3JldEVuY29kaW5nRm9yUmVsb2FkZXI=")},
	}
}

func createNamespace(t *testing.T, namespace string, client kubernetes.Interface) {
	_, err := client.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		t.Error("Failed to create namespace for testing", err)
	} else {
		logrus.Infof("Creating namespace for testing = %s", namespace)
	}
}

func deleteNamespace(t *testing.T, namespace string, client kubernetes.Interface) {
	err := client.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	if err != nil {
		t.Error("Failed to delete namespace that was created for testing", err)
	} else {
		logrus.Infof("Deleting namespace for testing = %s", namespace)
	}
}

func updateConfigmap(namespace string, configmapName string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string]string{"test.url": "www.stakater.com"},
	}
}

func updateSecret(namespace string, secretName string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels:    map[string]string{"firstLabel": "temp"},
		},
		Data: map[string][]byte{"test.url": []byte("dGVzdFVwZGF0ZWRTZWNyZXRFbmNvZGluZ0ZvclJlbG9hZGVy")},
	}
}
