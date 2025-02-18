package handler

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
	app "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PauseDeployment(deployment *app.Deployment, clients kube.Clients, deploymentName, namespace, pauseIntervalValue string) error {
	pauseDuration, err := ParsePauseDuration(pauseIntervalValue)
	if err != nil {
		return err
	}

	if !deployment.Spec.Paused {
		deployment.Spec.Paused = true
		logrus.Infof("Pausing Deployment '%s' in namespace '%s' for %s seconds", deploymentName, namespace, pauseDuration)

		if deployment.Annotations == nil {
			deployment.Annotations = make(map[string]string)
		}
		deployment.Annotations[options.PauseDeploymentTimeAnnotation] = time.Now().Format(time.RFC3339)

		CreateResumeTimer(deployment, clients, deploymentName, namespace, pauseDuration)
	} else {
		logrus.Infof("Deployment '%s' in namespace '%s' is already paused", deploymentName, namespace)
	}
	return nil
}

func CreateResumeTimer(deployment *app.Deployment, clients kube.Clients, deploymentName, namespace string, pauseDuration time.Duration) {
	time.AfterFunc(pauseDuration, func() {
		ResumeDeployment(deploymentName, namespace, clients)
	})
}

func ResumeDeployment(deploymentName, namespace string, clients kube.Clients) {
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		return
	}

	if !deployment.Spec.Paused {
		logrus.Infof("Deployment '%s' in namespace '%s' not paused. Skipping resume", deploymentName, namespace)
		return
	}

	pausedAtAnnotationValue := deployment.Annotations[options.PauseDeploymentTimeAnnotation]
	if pausedAtAnnotationValue == "" {
		logrus.Infof("Deployment '%s' in namespace '%s' was not paused by Reloader. Skipping resume", deploymentName, namespace)
		return
	}

	deployment.Spec.Paused = false
	delete(deployment.Annotations, options.PauseDeploymentTimeAnnotation)

	_, err = clients.KubernetesClient.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("Failed to resume deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
	}

	logrus.Infof("Successfully resumed deployment '%s' in namespace '%s'", deploymentName, namespace)
}

func ParsePauseDuration(pauseIntervalValue string) (time.Duration, error) {
	pauseDuration, err := time.ParseDuration(pauseIntervalValue)
	if err != nil {
		logrus.Warnf("Failed to parse pause interval value '%s': %v", pauseIntervalValue, err)
		return 0, err
	}
	return pauseDuration, nil
}
