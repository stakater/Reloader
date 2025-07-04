package callbacks

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
	app "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// GetPauseDuration returns the duration for which the deployment is paused
func GetPauseDuration(deployment app.Deployment) (string, time.Duration) {
	pauseDurationStr := deployment.Annotations[options.PauseDeploymentAnnotation]
	if pauseDurationStr == "" {
		return "", 0
	}
	pauseDuration, err := time.ParseDuration(pauseDurationStr)
	if err != nil {
		logrus.Warnf("Failed to parse pause interval value '%s': %v", pauseDurationStr, err)
		return "", 0
	}
	return pauseDurationStr, pauseDuration
}

// Handles the case where missing timers for deployments that have been paused by reloader.
// Could occur after new leader election or reloader restart
func HandleMissingTimer(deployment *app.Deployment, pauseDuration time.Duration, clients kube.Clients) {
	pauseStartTime, err := GetPauseStartTime(deployment)
	if err != nil {
		logrus.Errorf("Error parsing pause start time for deployment '%s' in namespace '%s': %v. Resuming deployment immediately",
			deployment.Name, deployment.Namespace, err)
		ResumeDeployment(deployment.Name, deployment.Namespace, clients)
		return
	}

	if pauseStartTime == nil {
		return
	}

	elapsedPauseTime := time.Since(*pauseStartTime)
	remainingPauseTime := pauseDuration - elapsedPauseTime

	if remainingPauseTime <= 0 {
		logrus.Infof("Pause period for deployment '%s' in namespace '%s' has expired. Resuming immediately",
			deployment.Name, deployment.Namespace)
		ResumeDeployment(deployment.Name, deployment.Namespace, clients)
		return
	}

	logrus.Infof("Creating missing timer for already paused deployment '%s' in namespace '%s' with remaining time %s",
		deployment.Name, deployment.Namespace, remainingPauseTime)
	CreateResumeTimer(deployment, clients, remainingPauseTime)
}

// CreateResumeTimer creates a timer to resume the deployment after the specified duration
func CreateResumeTimer(deployment *app.Deployment, clients kube.Clients, pauseDuration time.Duration) {
	timerKey := getTimerKey(deployment.Namespace, deployment.Name)

	// Check if there's an existing timer for this deployment
	if _, exists := activeTimers[timerKey]; exists {
		logrus.Debugf("Timer already exists for deployment '%s' in namespace '%s', Skipping creation",
			deployment.Name, deployment.Namespace)
		return
	}

	// Create and store the new timer
	timer := time.AfterFunc(pauseDuration, func() {
		ResumeDeployment(deployment.Name, deployment.Namespace, clients)
	})

	// Add the new timer to the map
	activeTimers[timerKey] = timer

	logrus.Debugf("Created pause timer for deployment '%s' in namespace '%s' with duration %s",
		deployment.Name, deployment.Namespace, pauseDuration)
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

	// Remove the timer
	timerKey := getTimerKey(namespace, deploymentName)
	if timer, exists := activeTimers[timerKey]; exists {
		timer.Stop()
		delete(activeTimers, timerKey)
		logrus.Debugf("Removed pause timer for deployment '%s' in namespace '%s'", deploymentName, namespace)
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

func CreatePausePatch(pauseIntervalValue string) string {
	timeAnnotation := time.Now().Format(time.RFC3339)
	return fmt.Sprintf(`{
		"spec": {"paused": true},
		"metadata": {"annotations": {"%s": "%s","%s": "%s" }}}`,
		options.PauseDeploymentTimeAnnotation, timeAnnotation,
		options.PauseDeploymentAnnotation, pauseIntervalValue)
}

func ApplyPauseAnnotation(deployment *app.Deployment, pauseIntervalValue string) {
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}
	deployment.Annotations[options.PauseDeploymentTimeAnnotation] = time.Now().Format(time.RFC3339)
	deployment.Annotations[options.PauseDeploymentAnnotation] = pauseIntervalValue
}

func ResumeTimerExists(deployment *app.Deployment) bool {
	timerKey := getTimerKey(deployment.Namespace, deployment.Name)
	_, exists := activeTimers[timerKey]
	if !exists {
		logrus.Warnf("Timer does not exist for deployment '%s' in namespace '%s'", deployment.Name, deployment.Namespace)
	}
	return exists
}

func PauseDeployment(deployment *app.Deployment, clients kube.Clients, doPatch bool) bool {
	pauseDurationString, pauseDuration := GetPauseDuration(*deployment)

	if pauseDurationString == "" || IsPaused(deployment) {
		return false
	}

	pausedByReloader := IsPausedByReloader(deployment)
	if pausedByReloader {
		// In case of leader election or reloader restart, check for missing timers
		return checkForMissingTimer(deployment, pauseDuration, clients)
	}

	return updateDeployment(deployment, clients, pauseDurationString, pauseDuration, doPatch)
}

func checkForMissingTimer(deployment *app.Deployment, pauseDuration time.Duration, clients kube.Clients) bool {
	if !ResumeTimerExists(deployment) {
		HandleMissingTimer(deployment, pauseDuration, clients)
	}
	return true
}

func updateDeployment(deployment *app.Deployment, clients kube.Clients, pauseDurationString string, pauseDuration time.Duration, doPatch bool) bool {
	logrus.Infof("Pausing Deployment '%s' in namespace '%s' for %s", deployment.Name, deployment.Namespace, pauseDuration)

	if doPatch {
		return pauseWithPatch(deployment, clients, pauseDurationString, pauseDuration)
	}

	updateDeploymentObject(deployment, pauseDurationString, pauseDuration)
	return true
}

func pauseWithPatch(deployment *app.Deployment, clients kube.Clients, pauseDurationString string, pauseDuration time.Duration) bool {
	pausePatch := CreatePausePatch(pauseDurationString)

	_, err := clients.KubernetesClient.AppsV1().Deployments(deployment.Namespace).Patch(
		context.TODO(),
		deployment.Name,
		types.StrategicMergePatchType,
		[]byte(pausePatch),
		meta_v1.PatchOptions{FieldManager: "Reloader"},
	)

	if err != nil {
		logrus.Errorf("Failed to pause deployment %s: %v", deployment.Name, err)
		return false
	}

	CreateResumeTimer(deployment, clients, pauseDuration)
	return true
}

func updateDeploymentObject(deployment *app.Deployment, pauseDurationString string, pauseDuration time.Duration) {
	ApplyPauseAnnotation(deployment, pauseDurationString)
	deployment.Spec.Paused = true
}
