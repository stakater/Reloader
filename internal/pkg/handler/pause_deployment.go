package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stakater/Reloader/pkg/options"
	app "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	patchtypes "k8s.io/apimachinery/pkg/types"
)

// Keeps track of currently active timers
var activeTimers = make(map[string]*time.Timer)

// Returns unique key for the activeTimers map
func getTimerKey(namespace, deploymentName string) string {
	return fmt.Sprintf("%s/%s", namespace, deploymentName)
}

// Checks if a deployment is currently paused
func IsPaused(deployment *app.Deployment) bool {
	return deployment.Spec.Paused
}

// Deployment paused by reloader ?
func IsPausedByReloader(deployment *app.Deployment) bool {
	if IsPaused(deployment) {
		pausedAtAnnotationValue := deployment.Annotations[options.PauseDeploymentTimeAnnotation]
		return pausedAtAnnotationValue != ""
	}
	return false
}

// Returns the time, the deployment was paused by reloader, nil otherwise
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

// ParsePauseDuration parses the pause interval value and returns a time.Duration
func ParsePauseDuration(pauseIntervalValue string) (time.Duration, error) {
	pauseDuration, err := time.ParseDuration(pauseIntervalValue)
	if err != nil {
		logrus.Warnf("Failed to parse pause interval value '%s': %v", pauseIntervalValue, err)
		return 0, err
	}
	return pauseDuration, nil
}

// Pauses a deployment for a specified duration and creates a timer to resume it
// after the specified duration
func PauseDeployment(deployment *app.Deployment, clients kube.Clients, namespace, pauseIntervalValue string) (*app.Deployment, error) {
	deploymentName := deployment.Name
	pauseDuration, err := ParsePauseDuration(pauseIntervalValue)

	if err != nil {
		return nil, err
	}

	if !IsPaused(deployment) {
		logrus.Infof("Pausing Deployment '%s' in namespace '%s' for %s", deploymentName, namespace, pauseDuration)

		deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

		pausePatch, err := CreatePausePatch()
		if err != nil {
			logrus.Errorf("Failed to create pause patch for deployment '%s': %v", deploymentName, err)
			return deployment, err
		}

		err = deploymentFuncs.PatchFunc(clients, namespace, deployment, patchtypes.StrategicMergePatchType, pausePatch)

		if err != nil {
			logrus.Errorf("Failed to patch deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
			return deployment, err
		}

		updatedDeployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})

		CreateResumeTimer(deployment, clients, namespace, pauseDuration)
		return updatedDeployment, err
	}

	if !IsPausedByReloader(deployment) {
		logrus.Infof("Deployment '%s' in namespace '%s' already paused", deploymentName, namespace)
		return deployment, nil
	}

	// Deployment has already been paused by reloader, check for timer
	logrus.Debugf("Deployment '%s' in namespace '%s' is already paused by reloader", deploymentName, namespace)

	timerKey := getTimerKey(namespace, deploymentName)
	_, timerExists := activeTimers[timerKey]

	if !timerExists {
		logrus.Warnf("Timer does not exist for already paused deployment '%s' in namespace '%s', creating new one",
			deploymentName, namespace)
		HandleMissingTimer(deployment, pauseDuration, clients, namespace)
	}
	return deployment, nil
}

// Handles the case where missing timers for deployments that have been paused by reloader.
// Could occur after new leader election or reloader restart
func HandleMissingTimer(deployment *app.Deployment, pauseDuration time.Duration, clients kube.Clients, namespace string) {
	deploymentName := deployment.Name
	pauseStartTime, err := GetPauseStartTime(deployment)
	if err != nil {
		logrus.Errorf("Error parsing pause start time for deployment '%s' in namespace '%s': %v. Resuming deployment immediately",
			deploymentName, namespace, err)
		ResumeDeployment(deployment, namespace, clients)
		return
	}

	if pauseStartTime == nil {
		return
	}

	elapsedPauseTime := time.Since(*pauseStartTime)
	remainingPauseTime := pauseDuration - elapsedPauseTime

	if remainingPauseTime <= 0 {
		logrus.Infof("Pause period for deployment '%s' in namespace '%s' has expired. Resuming immediately",
			deploymentName, namespace)
		ResumeDeployment(deployment, namespace, clients)
		return
	}

	logrus.Infof("Creating missing timer for already paused deployment '%s' in namespace '%s' with remaining time %s",
		deploymentName, namespace, remainingPauseTime)
	CreateResumeTimer(deployment, clients, namespace, remainingPauseTime)
}

// CreateResumeTimer creates a timer to resume the deployment after the specified duration
func CreateResumeTimer(deployment *app.Deployment, clients kube.Clients, namespace string, pauseDuration time.Duration) {
	deploymentName := deployment.Name
	timerKey := getTimerKey(namespace, deployment.Name)

	// Check if there's an existing timer for this deployment
	if _, exists := activeTimers[timerKey]; exists {
		logrus.Debugf("Timer already exists for deployment '%s' in namespace '%s', Skipping creation",
			deploymentName, namespace)
		return
	}

	// Create and store the new timer
	timer := time.AfterFunc(pauseDuration, func() {
		ResumeDeployment(deployment, namespace, clients)
	})

	// Add the new timer to the map
	activeTimers[timerKey] = timer

	logrus.Debugf("Created pause timer for deployment '%s' in namespace '%s' with duration %s",
		deploymentName, namespace, pauseDuration)
}

// ResumeDeployment resumes a deployment that has been paused by reloader
func ResumeDeployment(deployment *app.Deployment, namespace string, clients kube.Clients) {
	deploymentName := deployment.Name

	currentDeployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})

	if err != nil {
		logrus.Errorf("Failed to get deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		return
	}

	if !IsPausedByReloader(currentDeployment) {
		logrus.Infof("Deployment '%s' in namespace '%s' not paused by Reloader. Skipping resume", deploymentName, namespace)
		return
	}

	deploymentFuncs := GetDeploymentRollingUpgradeFuncs()

	resumePatch, err := CreateResumePatch()
	if err != nil {
		logrus.Errorf("Failed to create resume patch for deployment '%s': %v", deploymentName, err)
		return
	}

	// Remove the timer
	timerKey := getTimerKey(namespace, deploymentName)
	if timer, exists := activeTimers[timerKey]; exists {
		timer.Stop()
		delete(activeTimers, timerKey)
		logrus.Debugf("Removed pause timer for deployment '%s' in namespace '%s'", deploymentName, namespace)
	}

	err = deploymentFuncs.PatchFunc(clients, namespace, currentDeployment, patchtypes.StrategicMergePatchType, resumePatch)

	if err != nil {
		logrus.Errorf("Failed to resume deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		return
	}

	logrus.Infof("Successfully resumed deployment '%s' in namespace '%s'", deploymentName, namespace)
}

func CreatePausePatch() ([]byte, error) {
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"paused": true,
		},
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				options.PauseDeploymentTimeAnnotation: time.Now().Format(time.RFC3339),
			},
		},
	}

	return json.Marshal(patchData)
}

func CreateResumePatch() ([]byte, error) {
	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"paused": false,
		},
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				options.PauseDeploymentTimeAnnotation: nil,
			},
		},
	}

	return json.Marshal(patchData)
}
