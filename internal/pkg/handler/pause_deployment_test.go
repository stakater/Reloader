package handler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	testclient "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
)

func TestIsPaused(t *testing.T) {
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		paused     bool
	}{
		{
			name: "paused deployment",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
			},
			paused: true,
		},
		{
			name: "unpaused deployment",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			paused: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsPaused(test.deployment)
			assert.Equal(t, test.paused, result)
		})
	}
}

func TestIsPausedByReloader(t *testing.T) {
	tests := []struct {
		name             string
		deployment       *appsv1.Deployment
		pausedByReloader bool
	}{
		{
			name: "paused by reloader",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						options.PauseDeploymentTimeAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			pausedByReloader: true,
		},
		{
			name: "not paused by reloader",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			pausedByReloader: false,
		},
		{
			name: "not paused",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pausedByReloader: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pausedByReloader := IsPausedByReloader(test.deployment)
			assert.Equal(t, test.pausedByReloader, pausedByReloader)
		})
	}
}

func TestGetPauseStartTime(t *testing.T) {
	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	tests := []struct {
		name              string
		deployment        *appsv1.Deployment
		pausedByReloader  bool
		expectedStartTime time.Time
	}{
		{
			name: "valid pause time",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						options.PauseDeploymentTimeAnnotation: nowStr,
					},
				},
			},
			pausedByReloader:  true,
			expectedStartTime: now,
		},
		{
			name: "not paused by reloader",
			deployment: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pausedByReloader: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualStartTime, err := GetPauseStartTime(test.deployment)

			assert.NoError(t, err)

			if !test.pausedByReloader {
				assert.Nil(t, actualStartTime)
			} else {
				assert.NotNil(t, actualStartTime)
				assert.WithinDuration(t, test.expectedStartTime, *actualStartTime, time.Second)
			}
		})
	}
}

func TestParsePauseDuration(t *testing.T) {
	tests := []struct {
		name               string
		pauseIntervalValue string
		expectedDuration   time.Duration
		invalidDuration    bool
	}{
		{
			name:               "valid duration",
			pauseIntervalValue: "10s",
			expectedDuration:   10 * time.Second,
			invalidDuration:    false,
		},
		{
			name:               "valid minute duration",
			pauseIntervalValue: "2m",
			expectedDuration:   2 * time.Minute,
			invalidDuration:    false,
		},
		{
			name:               "invalid duration",
			pauseIntervalValue: "invalid",
			expectedDuration:   0,
			invalidDuration:    true,
		},
		{
			name:               "zero duration",
			pauseIntervalValue: "0s",
			expectedDuration:   0,
			invalidDuration:    true,
		},
		{
			name:               "negative duration",
			pauseIntervalValue: "-5m",
			expectedDuration:   0,
			invalidDuration:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualDuration, err := ParsePauseDuration(test.pauseIntervalValue)

			if test.invalidDuration {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedDuration, actualDuration)
			}
		})
	}
}

func TestHandleMissingTimerSimple(t *testing.T) {
	resetActiveTimers(t)

	tests := []struct {
		name           string
		deployment     *appsv1.Deployment
		shouldBePaused bool // Should be unpaused after HandleMissingTimer ?
	}{
		{
			name: "deployment paused by reloader, pause period has expired and no timer",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-deployment-1",
					Annotations: map[string]string{
						options.PauseDeploymentTimeAnnotation: time.Now().Add(-6 * time.Minute).Format(time.RFC3339),
						options.PauseDeploymentAnnotation:     "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
			},
			shouldBePaused: false,
		},
		{
			name: "deployment paused by reloader, pause period expires in the future and no timer",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-deployment-2",
					Annotations: map[string]string{
						options.PauseDeploymentTimeAnnotation: time.Now().Add(1 * time.Minute).Format(time.RFC3339),
						options.PauseDeploymentAnnotation:     "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
			},
			shouldBePaused: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := testclient.NewClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			_, err := fakeClient.AppsV1().Deployments("default").Create(
				context.TODO(),
				test.deployment,
				metav1.CreateOptions{})
			assert.NoError(t, err, "Expected no error when creating deployment")

			pauseDuration, _ := ParsePauseDuration(test.deployment.Annotations[options.PauseDeploymentAnnotation])
			HandleMissingTimer(test.deployment, pauseDuration, clients, "default")

			updatedDeployment, _ := fakeClient.AppsV1().Deployments("default").Get(context.TODO(), test.deployment.Name, metav1.GetOptions{})

			assert.Equal(t, test.shouldBePaused, updatedDeployment.Spec.Paused,
				"Deployment should have correct paused state after timer expiration")

			if test.shouldBePaused {
				pausedAtAnnotationValue := updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.NotEmpty(t, pausedAtAnnotationValue,
					"Pause annotation should be present and contain a value when deployment is paused")
			}
		})
	}
}

// resetActiveTimers stops and removes all resume timers when the test ends.
func resetActiveTimers(t *testing.T) {
	t.Cleanup(func() {
		activeTimersMu.Lock()
		defer activeTimersMu.Unlock()
		for key, timer := range activeTimers {
			timer.Stop()
			delete(activeTimers, key)
		}
	})
}

// useShortResumeRetryBackoff makes failed resumes retry within milliseconds.
func useShortResumeRetryBackoff(t *testing.T) {
	original := resumeRetryBackoff
	resumeRetryBackoff = wait.Backoff{Duration: 10 * time.Millisecond, Factor: 2, Steps: math.MaxInt32, Cap: 50 * time.Millisecond}
	t.Cleanup(func() { resumeRetryBackoff = original })
}

func hasResumeTimer(timerKey string) bool {
	activeTimersMu.Lock()
	defer activeTimersMu.Unlock()
	_, exists := activeTimers[timerKey]
	return exists
}

func newExpiredPausedDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				options.PauseDeploymentTimeAnnotation: time.Now().Add(-6 * time.Minute).Format(time.RFC3339),
				options.PauseDeploymentAnnotation:     "5m",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Paused: true,
		},
	}
}

func TestResumeDeploymentRetriesUntilResumed(t *testing.T) {
	tests := []struct {
		name string
		verb string
	}{
		{
			name: "API server unavailable",
			verb: "get",
		},
		{
			name: "resume write rejected by a webhook",
			verb: "patch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetActiveTimers(t)
			useShortResumeRetryBackoff(t)

			deployment := newExpiredPausedDeployment()
			fakeClient := testclient.NewClientset(deployment)
			clients := kube.Clients{KubernetesClient: fakeClient}

			var failures atomic.Int32
			fakeClient.PrependReactor(test.verb, "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
				if failures.Add(1) <= 2 {
					return true, nil, errors.New("connection refused")
				}
				return false, nil, nil
			})

			ResumeDeployment(deployment, "default", clients)

			timerKey := getTimerKey("default", deployment.Name)
			assert.True(t, hasResumeTimer(timerKey), "A retry timer should be kept while the resume is pending")

			assert.Eventually(t, func() bool {
				current, err := fakeClient.AppsV1().Deployments("default").Get(context.TODO(), deployment.Name, metav1.GetOptions{})
				return err == nil && !current.Spec.Paused
			}, 5*time.Second, 5*time.Millisecond, "Deployment should be resumed by the retry timer")

			updatedDeployment, err := fakeClient.AppsV1().Deployments("default").Get(context.TODO(), deployment.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			_, hasPauseAnnotation := updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation]
			assert.False(t, hasPauseAnnotation, "Pause annotation should be cleared after a successful resume")
			assert.False(t, hasResumeTimer(timerKey), "Timer should be removed after a successful resume")
		})
	}
}

func TestResumeDeploymentStopsRetrying(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, client *testclient.Clientset, deployment *appsv1.Deployment)
	}{
		{
			name: "deployment deleted",
			change: func(t *testing.T, client *testclient.Clientset, deployment *appsv1.Deployment) {
				err := client.AppsV1().Deployments("default").Delete(context.TODO(), deployment.Name, metav1.DeleteOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "deployment no longer paused by reloader",
			change: func(t *testing.T, client *testclient.Clientset, deployment *appsv1.Deployment) {
				resumed := deployment.DeepCopy()
				resumed.Spec.Paused = false
				_, err := client.AppsV1().Deployments("default").Update(context.TODO(), resumed, metav1.UpdateOptions{})
				assert.NoError(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetActiveTimers(t)
			useShortResumeRetryBackoff(t)

			deployment := newExpiredPausedDeployment()
			fakeClient := testclient.NewClientset(deployment)
			clients := kube.Clients{KubernetesClient: fakeClient}

			var gets atomic.Int32
			fakeClient.PrependReactor("get", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
				if gets.Add(1) == 1 {
					return true, nil, errors.New("connection refused")
				}
				return false, nil, nil
			})

			ResumeDeployment(deployment, "default", clients)
			test.change(t, fakeClient, deployment)

			timerKey := getTimerKey("default", deployment.Name)
			assert.Eventually(t, func() bool {
				return gets.Load() >= 2 && !hasResumeTimer(timerKey)
			}, 5*time.Second, 5*time.Millisecond, "Retries should stop once the resume is no longer needed")

			time.Sleep(100 * time.Millisecond)
			assert.Equal(t, int32(2), gets.Load(), "No further retries should be scheduled")
			assert.False(t, hasResumeTimer(timerKey))
		})
	}
}

func TestResumeDeploymentKeepsTimerFromConcurrentPause(t *testing.T) {
	resetActiveTimers(t)

	deployment := newExpiredPausedDeployment()
	fakeClient := testclient.NewClientset(deployment)
	clients := kube.Clients{KubernetesClient: fakeClient}
	timerKey := getTimerKey("default", deployment.Name)

	// A new change pauses the deployment again while the resume write is in
	// flight, and the resume write then fails.
	var pauseTimer *time.Timer
	fakeClient.PrependReactor("patch", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
		CreateResumeTimer(deployment, clients, "default", time.Hour)
		activeTimersMu.Lock()
		pauseTimer = activeTimers[timerKey]
		activeTimersMu.Unlock()
		return true, nil, errors.New("connection refused")
	})

	ResumeDeployment(deployment, "default", clients)

	activeTimersMu.Lock()
	timer, exists := activeTimers[timerKey]
	activeTimersMu.Unlock()
	assert.True(t, exists, "The timer from the new pause should be kept")
	assert.Same(t, pauseTimer, timer, "The retry should not replace the timer from the new pause")
}

func TestResumeDeploymentRecoversAfterManualResume(t *testing.T) {
	pausedDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			Annotations: map[string]string{
				options.PauseDeploymentTimeAnnotation: time.Now().Add(-6 * time.Minute).Format(time.RFC3339),
				options.PauseDeploymentAnnotation:     "5m",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Paused: true,
		},
	}
	// The deployment was un-paused by hand before the resume timer fired.
	resumedDeployment := pausedDeployment.DeepCopy()
	resumedDeployment.Spec.Paused = false
	fakeClient := testclient.NewClientset(resumedDeployment)
	clients := kube.Clients{KubernetesClient: fakeClient}

	resetActiveTimers(t)
	timerKey := getTimerKey("default", pausedDeployment.Name)
	staleTimer := time.NewTimer(time.Hour)
	activeTimersMu.Lock()
	activeTimers[timerKey] = staleTimer
	activeTimersMu.Unlock()

	ResumeDeployment(pausedDeployment, "default", clients)

	assert.False(t, hasResumeTimer(timerKey), "Timer should be removed even if the deployment is no longer paused by reloader")

	// The next change pauses the deployment again and must get a fresh resume timer.
	_, err := PauseDeployment(resumedDeployment, clients, "default", resumedDeployment.Annotations[options.PauseDeploymentAnnotation])
	assert.NoError(t, err)

	updatedDeployment, err := fakeClient.AppsV1().Deployments("default").Get(context.TODO(), resumedDeployment.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, updatedDeployment.Spec.Paused, "Deployment should be paused by the new change")

	activeTimersMu.Lock()
	timer, timerExists := activeTimers[timerKey]
	activeTimersMu.Unlock()
	assert.True(t, timerExists, "A resume timer should exist for the new pause")
	assert.NotSame(t, staleTimer, timer, "The resume timer should not be the stale one")
}

func TestPauseDeployment(t *testing.T) {
	tests := []struct {
		name               string
		deployment         *appsv1.Deployment
		expectedError      bool
		expectedPaused     bool
		expectedAnnotation bool // Should have pause time annotation
		pauseInterval      string
	}{
		{
			name: "deployment without pause annotation",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment",
					Annotations: map[string]string{},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			expectedError:      true,
			expectedPaused:     false,
			expectedAnnotation: false,
			pauseInterval:      "",
		},
		{
			name: "deployment already paused but not by reloader",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-deployment",
					Annotations: map[string]string{
						options.PauseDeploymentAnnotation: "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: true,
				},
			},
			expectedError:      false,
			expectedPaused:     true,
			expectedAnnotation: false,
			pauseInterval:      "5m",
		},
		{
			name: "deployment unpaused that needs to be paused by reloader",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-deployment-3",
					Annotations: map[string]string{
						options.PauseDeploymentAnnotation: "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			expectedError:      false,
			expectedPaused:     true,
			expectedAnnotation: true,
			pauseInterval:      "5m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := testclient.NewClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			_, err := fakeClient.AppsV1().Deployments("default").Create(
				context.TODO(),
				test.deployment,
				metav1.CreateOptions{})
			assert.NoError(t, err, "Expected no error when creating deployment")

			updatedDeployment, err := PauseDeployment(test.deployment, clients, "default", test.pauseInterval)
			if test.expectedError {
				assert.Error(t, err, "Expected an error pausing the deployment")
				return
			} else {
				assert.NoError(t, err, "Expected no error pausing the deployment")
			}

			assert.Equal(t, test.expectedPaused, updatedDeployment.Spec.Paused,
				"Deployment should have correct paused state after pause")

			if test.expectedAnnotation {
				pausedAtAnnotationValue := updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.NotEmpty(t, pausedAtAnnotationValue,
					"Pause annotation should be present and contain a value when deployment is paused")
			} else {
				pausedAtAnnotationValue := updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.Empty(t, pausedAtAnnotationValue,
					"Pause annotation should not be present when deployment has not been paused by reloader")
			}
		})
	}
}

// Simple helper function for test cases
func FindDeploymentByName(deployments []runtime.Object, deploymentName string) (*appsv1.Deployment, error) {
	for _, deployment := range deployments {
		accessor, err := meta.Accessor(deployment)
		if err != nil {
			return nil, fmt.Errorf("error getting accessor for item: %w", err)
		}
		if accessor.GetName() == deploymentName {
			deploymentObj, ok := deployment.(*appsv1.Deployment)
			if !ok {
				return nil, fmt.Errorf("failed to cast to Deployment")
			}
			return deploymentObj, nil
		}
	}
	return nil, fmt.Errorf("deployment '%s' not found", deploymentName)
}

func TestResumeTimersConcurrentAccess(t *testing.T) {
	// Each controller worker and each resume timer runs in its own goroutine,
	// so pausing and resuming different deployments must not race on activeTimers.
	resetActiveTimers(t)
	clients := kube.Clients{KubernetesClient: testclient.NewClientset()}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-deployment-%d", i),
				Namespace: "default",
			},
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				CreateResumeTimer(deployment, clients, "default", time.Hour)
				ResumeDeployment(deployment, "default", clients)
			}
		}()
	}
	wg.Wait()

	activeTimersMu.Lock()
	defer activeTimersMu.Unlock()
	assert.Empty(t, activeTimers, "All resume timers should be removed")
}
