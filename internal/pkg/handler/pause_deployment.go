package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	app "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	patchtypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
)

// resumeRetryBackoff controls how often a failed resume is retried. Retries
// continue until the resume succeeds or is no longer needed.
var resumeRetryBackoff = wait.Backoff{
	Duration: 5 * time.Second,
	Factor:   2,
	Jitter:   0.1,
	Steps:    math.MaxInt32,
	Cap:      60 * time.Second,
}

// Keeps track of currently active timers. Controller workers and resume
// timers run in separate goroutines, so access is guarded by activeTimersMu.
var (
	activeTimers   = make(map[string]*time.Timer)
	activeTimersMu sync.Mutex
)

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
	if pauseDuration <= 0 {
		err = fmt.Errorf("pause interval must be positive, got '%s'", pauseIntervalValue)
		logrus.Warn(err)
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
	activeTimersMu.Lock()
	_, timerExists := activeTimers[timerKey]
	activeTimersMu.Unlock()

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

	activeTimersMu.Lock()
	defer activeTimersMu.Unlock()

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
	backoff := resumeRetryBackoff
	resumeDeployment(deployment.Name, namespace, clients, &backoff)
}

func resumeDeployment(deploymentName, namespace string, clients kube.Clients, backoff *wait.Backoff) {
	// Remove the timer first: a pause that happens while this resume is in
	// flight must be able to create its own timer. If this attempt fails, the
	// retry is only scheduled when no such timer exists.
	timerKey := getTimerKey(namespace, deploymentName)
	activeTimersMu.Lock()
	if timer, exists := activeTimers[timerKey]; exists {
		timer.Stop()
		delete(activeTimers, timerKey)
		logrus.Debugf("Removed pause timer for deployment '%s' in namespace '%s'", deploymentName, namespace)
	}
	activeTimersMu.Unlock()

	currentDeployment, err := clients.KubernetesClient.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Infof("Deployment '%s' in namespace '%s' no longer exists. Skipping resume", deploymentName, namespace)
			return
		}
		logrus.Errorf("Failed to get deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		scheduleResumeRetry(deploymentName, namespace, clients, backoff)
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
		scheduleResumeRetry(deploymentName, namespace, clients, backoff)
		return
	}

	err = deploymentFuncs.PatchFunc(clients, namespace, currentDeployment, patchtypes.StrategicMergePatchType, resumePatch)

	if err != nil {
		logrus.Errorf("Failed to resume deployment '%s' in namespace '%s': %v", deploymentName, namespace, err)
		scheduleResumeRetry(deploymentName, namespace, clients, backoff)
		return
	}

	logrus.Infof("Successfully resumed deployment '%s' in namespace '%s'", deploymentName, namespace)
}

// scheduleResumeRetry retries a failed resume after the next backoff step.
// If a pause created a new timer in the meantime, that timer resumes the
// deployment and no retry is needed.
func scheduleResumeRetry(deploymentName, namespace string, clients kube.Clients, backoff *wait.Backoff) {
	timerKey := getTimerKey(namespace, deploymentName)

	activeTimersMu.Lock()
	defer activeTimersMu.Unlock()

	if _, exists := activeTimers[timerKey]; exists {
		logrus.Debugf("Timer already exists for deployment '%s' in namespace '%s', Skipping resume retry",
			deploymentName, namespace)
		return
	}

	delay := backoff.Step()
	activeTimers[timerKey] = time.AfterFunc(delay, func() {
		resumeDeployment(deploymentName, namespace, clients, backoff)
	})

	logrus.Infof("Scheduled resume retry for deployment '%s' in namespace '%s' in %s",
		deploymentName, namespace, delay)
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
