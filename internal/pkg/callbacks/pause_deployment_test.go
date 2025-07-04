package callbacks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/pkg/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	app "k8s.io/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
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
						options.PauseDeploymentAnnotation:     "10s",
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

			require.NoError(t, err)

			if !test.pausedByReloader {
				assert.Nil(t, actualStartTime)
			} else {
				require.NotNil(t, actualStartTime)
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
				require.NoError(t, err)
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
					Name:      "test-deployment-1",
					Namespace: "default",
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
					Name:      "test-deployment-2",
					Namespace: "default",
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
			fakeClient := testclient.NewSimpleClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			_, err := fakeClient.AppsV1().Deployments("default").Create(
				context.TODO(),
				test.deployment,
				metav1.CreateOptions{})
			require.NoError(t, err, "Expected no error when creating deployment")

			pauseDuration, _ := ParsePauseDuration(test.deployment.Annotations[options.PauseDeploymentAnnotation])
			HandleMissingTimer(test.deployment, pauseDuration, clients)

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
		shouldBePaused     bool // Return value of PauseDeployment
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
			shouldBePaused:     false,
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
			shouldBePaused:     false,
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
			shouldBePaused:     true,
			expectedAnnotation: true,
			pauseInterval:      "5m",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := testclient.NewSimpleClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			hasBeenPaused := PauseDeployment(test.deployment, clients, false)

			require.Equal(t, test.shouldBePaused, hasBeenPaused,
				"PauseDeployment should have returned correct paused state")

			assert.Equal(t, test.expectedPaused, test.deployment.Spec.Paused,
				"Deployment should have correct paused state after pause")

			if test.expectedAnnotation {
				pausedAtAnnotationValue := test.deployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.NotEmpty(t, pausedAtAnnotationValue,
					"Pause annotation should be present and contain a value when deployment is paused")
			} else {
				pausedAtAnnotationValue := test.deployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.Empty(t, pausedAtAnnotationValue,
					"Pause annotation should not be present when deployment has not been paused by reloader")
			}
		})
	}
}

func TestPauseWithPatch(t *testing.T) {
	tests := []struct {
		name                string
		deployment          *appsv1.Deployment
		pauseDurationString string
		pauseDuration       time.Duration
		expectedResult      bool
		expectTimer         bool
		shouldPatchSucceed  bool
	}{
		{
			name: "successful pause deployment with patch",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-1",
					Namespace: "default",
					Annotations: map[string]string{
						options.PauseDeploymentAnnotation: "5m",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "5m",
			pauseDuration:       5 * time.Minute,
			expectedResult:      true,
			expectTimer:         true,
			shouldPatchSucceed:  true,
		},
		{
			name: "deployment without annotations",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment-2",
					Namespace:   "default",
					Annotations: nil,
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "30s",
			pauseDuration:       30 * time.Second,
			expectedResult:      true,
			expectTimer:         true,
			shouldPatchSucceed:  true,
		},
	}

	for _, test := range tests {
		// Clean up any timers at the end of each test
		defer func() {
			for key, timer := range activeTimers {
				timer.Stop()
				delete(activeTimers, key)
			}
		}()

		t.Run(test.name, func(t *testing.T) {
			fakeClient := testclient.NewSimpleClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			_, err := fakeClient.AppsV1().Deployments(test.deployment.Namespace).Create(
				context.TODO(),
				test.deployment,
				metav1.CreateOptions{})
			require.NoError(t, err, "Expected no error when creating deployment")

			result := PauseWithPatch(test.deployment, clients, test.pauseDurationString, test.pauseDuration)

			require.Equal(t, test.expectedResult, result,
				"PauseWithPatch should return correct state")

			// Check if timer was created
			timerKey := getTimerKey(test.deployment.Namespace, test.deployment.Name)
			_, timerExists := activeTimers[timerKey]
			assert.Equal(t, test.expectTimer, timerExists,
				"Timer should exist if pause was successful")

			if test.shouldPatchSucceed {
				updatedDeployment, err := fakeClient.AppsV1().Deployments(test.deployment.Namespace).Get(
					context.TODO(),
					test.deployment.Name,
					metav1.GetOptions{})

				require.NoError(t, err, "Should be able to get updated deployment")
				assert.True(t, updatedDeployment.Spec.Paused, "Deployment should be paused after patch")

				pauseTimeAnnotation := updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.NotEmpty(t, pauseTimeAnnotation, "Pause time annotation should be set")

				pauseIntervalAnnotation := updatedDeployment.Annotations[options.PauseDeploymentAnnotation]
				assert.Equal(t, test.pauseDurationString, pauseIntervalAnnotation, "Pause interval annotation should match")
			}
		})
	}
}

func TestApplyPauseToDeployment(t *testing.T) {
	tests := []struct {
		name                string
		deployment          *appsv1.Deployment
		pauseDurationString string
		pauseDuration       time.Duration
		expectPaused        bool
		expectAnnotations   bool
	}{
		{
			name: "apply pause to unpaused deployment",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-1",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "5m",
			pauseDuration:       5 * time.Minute,
			expectPaused:        true,
			expectAnnotations:   true,
		},
		{
			name: "apply pause to deployment with no annotations map",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-deployment-3",
					Namespace:   "default",
					Annotations: nil,
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "30s",
			pauseDuration:       30 * time.Second,
			expectPaused:        true,
			expectAnnotations:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ApplyPauseToDeployment(test.deployment, test.pauseDurationString, test.pauseDuration)

			require.Equal(t, test.expectPaused, test.deployment.Spec.Paused,
				"Deployment should be paused after applying pause")

			if test.expectAnnotations {
				require.NotNil(t, test.deployment.Annotations,
					"Annotations map should be created if it didn't exist")

				pauseTimeAnnotation := test.deployment.Annotations[options.PauseDeploymentTimeAnnotation]
				assert.NotEmpty(t, pauseTimeAnnotation,
					"Pause time annotation should be set")

				pauseIntervalAnnotation := test.deployment.Annotations[options.PauseDeploymentAnnotation]
				assert.Equal(t, test.pauseDurationString, pauseIntervalAnnotation,
					"Pause interval annotation should match the input")
			}
		})
	}
}

func TestPerformUpdate(t *testing.T) {
	tests := []struct {
		name                string
		deployment          *appsv1.Deployment
		pauseDurationString string
		pauseDuration       time.Duration
		doPatch             bool
		updateSucceeded     bool
	}{
		{
			name: "update deployment with patch",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-patch",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "5m",
			pauseDuration:       5 * time.Minute,
			doPatch:             true,
			updateSucceeded:     true,
		},
		{
			name: "update deployment without patch",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-no-patch",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Paused: false,
				},
			},
			pauseDurationString: "10m",
			pauseDuration:       10 * time.Minute,
			doPatch:             false,
			updateSucceeded:     true,
		},
	}

	for _, test := range tests {
		defer func() {
			for key, timer := range activeTimers {
				timer.Stop()
				delete(activeTimers, key)
			}
		}()

		t.Run(test.name, func(t *testing.T) {
			fakeClient := testclient.NewSimpleClientset()
			clients := kube.Clients{
				KubernetesClient: fakeClient,
			}

			_, err := fakeClient.AppsV1().Deployments(test.deployment.Namespace).Create(
				context.TODO(),
				test.deployment,
				metav1.CreateOptions{})
			require.Nil(t, err, "Expected no error when creating deployment")

			result := PerformUpdate(test.deployment, clients, test.pauseDurationString, test.pauseDuration, test.doPatch)

			require.Equal(t, test.updateSucceeded, result,
				"PerformUpdate should return correct result")

			if test.doPatch {
				// For patch, get deployment
				updatedDeployment, err := fakeClient.AppsV1().Deployments(test.deployment.Namespace).Get(
					context.TODO(),
					test.deployment.Name,
					metav1.GetOptions{})

				require.NoError(t, err, "Should be able to get updated deployment")
				assert.True(t, updatedDeployment.Spec.Paused, "Deployment should be paused")

				assert.NotEmpty(t, updatedDeployment.Annotations[options.PauseDeploymentTimeAnnotation],
					"Pause time annotation should be present")
				assert.Equal(t, test.pauseDurationString, updatedDeployment.Annotations[options.PauseDeploymentAnnotation],
					"Pause interval annotation should match")
			} else {
				assert.True(t, test.deployment.Spec.Paused, "Deployment should be paused")

				assert.NotEmpty(t, test.deployment.Annotations[options.PauseDeploymentTimeAnnotation],
					"Pause time annotation should be present")
				assert.Equal(t, test.pauseDurationString, test.deployment.Annotations[options.PauseDeploymentAnnotation],
					"Pause interval annotation should match")
			}
		})
	}
}

// Simple helper function for test cases
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
