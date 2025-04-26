package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
	app "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// IsPaused checks if a deployment is currently paused
func IsPaused(deployment *app.Deployment) bool {
	return deployment.Spec.Paused
}

// IsPausedByReloader checks if a deployment was paused by reloader
func IsPausedByReloader(deployment *app.Deployment) bool {
	if !deployment.Spec.Paused {
		return false
	}

	pausedAtAnnotationValue := deployment.Annotations[options.PauseDeploymentTimeAnnotation]
	return pausedAtAnnotationValue != ""
}

// FindDeploymentByName locates a deployment by name from a list of api objects
func FindDeploymentByName(deployments []runtime.Object, deploymentName string) (*app.Deployment, error) {
	for _, deployment := range deployments {
		accessor, err := meta.Accessor(deployment)
		if err != nil {
			return nil, fmt.Errorf("error getting accessor for item: %v", err)
		}
		if accessor.GetName() == deploymentName {
			deploymentObj, ok := deployment.(*app.Deployment)
			if !ok {
				return nil, fmt.Errorf("failed to cast to Deployment")
			}
			return deploymentObj, nil
		}
	}
	return nil, fmt.Errorf("deployment '%s' not found", deploymentName)
}

// GetPauseStartTime returns when the deployment was paused by reloader, nil otherwise
func GetPauseStartTime(deployment *app.Deployment) (*time.Time, error) {
	if !IsPausedByReloader(deployment) {
		return nil, nil
	}

	pausedAtStr := deployment.Annotations[options.PauseDeploymentTimeAnnotation]
	parsedTime, err := time.Parse(time.RFC3339, pausedAtStr)
	if err != nil {
		return nil, err
	}

	return &parsedTime, nil
}

// PauseDeployment pauses a deployment for a specified duration and creates a timer to resume it
// after the specified duration
func PauseDeployment(deployment *app.Deployment, clients kube.Clients, deploymentName, namespace, pauseIntervalValue string) error {
	pauseDuration, err := ParsePauseDuration(pauseIntervalValue)
	if err != nil {
		return err
	}

	if !IsPaused(deployment) {
		deployment.Spec.Paused = true
		logrus.Infof("Pausing Deployment '%s' in namespace '%s' for %s", deploymentName, namespace, pauseDuration)

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

// CreateResumeTimer creates a timer to resume the deployment after the specified duration
func CreateResumeTimer(deployment *app.Deployment, clients kube.Clients, deploymentName, namespace string, pauseDuration time.Duration) {
	time.AfterFunc(pauseDuration, func() {
		ResumeDeployment(deploymentName, namespace, clients)
	})
}

// ResumeDeployment resumes a deployment that has been paused by reloader
func ResumeDeployment(deploymentName, namespace string, clients kube.Clients) {
	deployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		return
	}

	if !IsPausedByReloader(deployment) {
		logrus.Infof("Deployment '%s' in namespace '%s' not paused by Reloader. Skipping resume", deploymentName, namespace)
		return
	}

	deployment.Spec.Paused = false
	delete(deployment.Annotations, options.PauseDeploymentTimeAnnotation)

	_, err = clients.KubernetesClient.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("Failed to resume deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		return
	}

	logrus.Infof("Successfully resumed deployment '%s' in namespace '%s'", deploymentName, namespace)
}

// ParsePauseDuration parses the pause interval value and returns a time.Duration
func ParsePauseDuration(pauseIntervalValue string) (time.Duration, error) {
	pauseDuration, err := time.ParseDuration(pauseIntervalValue)
	if err != nil {
		logrus.Warnf("Failed to parse pause interval value '%s': %v", pauseIntervalValue, err)
		return 0, err
	}
	return pauseDuration, nil
}
