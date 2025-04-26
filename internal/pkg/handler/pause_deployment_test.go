package handler

import (
	"testing"
	"time"

	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestFindDeploymentByName(t *testing.T) {
	testDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
	}

	additionalDeployment1 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-matching-deployment-1",
		},
	}

	additionalDeployment2 := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "non-matching-deployment-2",
		},
	}

	tests := []struct {
		name               string
		deployments        []runtime.Object
		deploymentName     string
		expectedDeployment *appsv1.Deployment
		deploymentFound    bool
	}{
		{
			name: "deployment found",
			deployments: []runtime.Object{
				additionalDeployment1,
				testDeployment,
				additionalDeployment2,
			},
			deploymentName:     "test-deployment",
			expectedDeployment: testDeployment,
			deploymentFound:    true,
		},
		{
			name: "deployment not found",
			deployments: []runtime.Object{
				additionalDeployment1,
				testDeployment,
				additionalDeployment2,
			},
			deploymentName:     "non-existent-deployment",
			expectedDeployment: nil,
			deploymentFound:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			foundDeployment, err := FindDeploymentByName(test.deployments, test.deploymentName)

			if test.deploymentFound {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedDeployment, foundDeployment)
			} else {
				assert.Error(t, err)
				assert.Nil(t, foundDeployment)
			}
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
