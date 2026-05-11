package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"

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
		// Clean up any timers at the end of the test
		defer func() {
			for key, timer := range activeTimers {
				timer.Stop()
				delete(activeTimers, key)
			}
		}()

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
