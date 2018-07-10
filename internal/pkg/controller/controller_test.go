package controller

import (
	"time"
	"math/rand"

	"github.com/stakater/Reloader/pkg/kube"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	client, _     = kube.GetClient()
	configmapNamePrefix = "testconfigmap-reloader"
	letters       = []rune("abcdefghijklmnopqrstuvwxyz")
)

func randSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Creating a Controller for Updating Pod with Default Action without Resources so messages printed
/*func TestControllerForUpdatePodShouldUpdateDefaultAction(t *testing.T) {
	controller, err := NewController(client, "configMaps", &v1.ConfigMap{})
	if err != nil {
		logrus.Infof("Unable to create NewController error = %v", err)
		return
	}
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(1, stop)
	time.Sleep(10 * time.Second)
	namespace := "test"
	configmapName := configmapNamePrefix + "-withoutresources-update-" + randSeq(5)
	configmapClient := client.CoreV1().ConfigMaps(namespace)
	configmap := initConfigmap(namespace, configmapName)
	configmap, err = configmapClient.Create(configmap)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Created Configmap %q.\n", configmap.GetObjectMeta().GetName())
	time.Sleep(10 * time.Second)

	logrus.Infof("Updating Configmap %q.\n", configmap.GetObjectMeta().GetName())
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configmap, err = configmapClient.Get(configmapName, metav1.GetOptions{})
		if err != nil {

		}
		configmap = updateConfigmap(namespace, configmapName)
		_, updateErr := configmapClient.Update(configmap)
		return updateErr
	})

	
	// TODO: Add functionality to verify reloader functionality here
	
	if retryErr != nil {
		controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
		panic(retryErr)
	}
	time.Sleep(10 * time.Second)
	logrus.Infof("Deleting Pod %q.\n", configmap.GetObjectMeta().GetName())
	controller.client.CoreV1().ConfigMaps(namespace).Delete(configmapName, &metav1.DeleteOptions{})
	time.Sleep(15 * time.Second)
}*/

func initConfigmap(namespace string, configmapName string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels: map[string]string{"firstLabel": "temp"},
		},
	}
}

func updateConfigmap(namespace string, configmapName string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configmapName,
			Namespace: namespace,
			Labels: map[string]string{"firstLabel": "updated"},
		},
	}
}
