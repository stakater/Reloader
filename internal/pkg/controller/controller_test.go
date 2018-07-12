package controller

import (
	"math/rand"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	client, err         = kube.GetClient()
	configmapNamePrefix = "testconfigmap-reloader"
	secretNamePrefix = "testsecret-reloader"
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
	namespace := "test-reloader"
	createNamespace(t, namespace)
	defer deleteNamespace(t, namespace)

	controller, err := NewController(client, "configMaps", namespace)
	if err != nil {
		logrus.Infof("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	configmapName := configmapNamePrefix + "-update-" + randSeq(5)
	configmapClient := client.CoreV1().ConfigMaps(namespace)
	configmap := initConfigmap(namespace, configmapName)
	configmap, err = configmapClient.Create(configmap)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Created Configmap %q.\n", configmap.GetObjectMeta().GetName())
	time.Sleep(10 * time.Second)

	logrus.Infof("Updating Configmap %q.\n", configmap.GetObjectMeta().GetName())
	configmap, err = configmapClient.Get(configmapName, metav1.GetOptions{})
	if err != nil {

	}
	configmap = updateConfigmap(namespace, configmapName)
	_, updateErr := configmapClient.Update(configmap)

	// TODO: Add functionality to verify reloader functionality here

	if updateErr != nil {
		controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		panic(updateErr)
	}
	time.Sleep(10 * time.Second)
	logrus.Infof("Deleting Configmap %q.\n", configmap.GetObjectMeta().GetName())
	controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	time.Sleep(15 * time.Second)
}

func TestControllerForUpdatingSecretShouldUpdateDeployment(t *testing.T) {
	namespace := "test-reloader-secrets"
	createNamespace(t, namespace)
	defer deleteNamespace(t, namespace)

	controller, err := NewController(client, "secrets", namespace)
	if err != nil {
		logrus.Infof("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)

	secretName := secretNamePrefix + "-update-" + randSeq(5)
	secretClient := client.CoreV1().Secrets(namespace)
	secret := initSecret(namespace, secretName)
	secret, err = secretClient.Create(secret)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Created Secret %q.\n", secret.GetObjectMeta().GetName())
	time.Sleep(10 * time.Second)

	logrus.Infof("Updating Secret %q.\n", secret.GetObjectMeta().GetName())
	secret, err = secretClient.Get(secretName, metav1.GetOptions{})
	if err != nil {

	}
	secret = updateSecret(namespace, secretName)
	_, updateErr := secretClient.Update(secret)

	// TODO: Add functionality to verify reloader functionality here

	if updateErr != nil {
		controller.client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
		panic(updateErr)
	}
	time.Sleep(10 * time.Second)
	logrus.Infof("Deleting Secret %q.\n", secret.GetObjectMeta().GetName())
	controller.client.CoreV1().Secrets(namespace).Delete(secretName, &metav1.DeleteOptions{})
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

func createNamespace(t *testing.T, namespace string) {
	_, err := client.CoreV1().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	if err != nil {
		t.Error("Failed to create namespace for testing", err)
	} else {
		logrus.Infof("Creating namespace for testing = %s", namespace)
	}
}

func deleteNamespace(t *testing.T, namespace string) {
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
