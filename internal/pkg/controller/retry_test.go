package controller

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/reload"
	"github.com/stakater/Reloader/internal/pkg/workload"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateWorkloadWithRetry_SwitchCases(t *testing.T) {
	// Test that the switch statement correctly identifies workload types
	// Note: Full integration tests require a fake k8s client, so we just test type detection

	tests := []struct {
		name         string
		workload     workload.WorkloadAccessor
		expectedKind workload.Kind
	}{
		{
			name: "deployment workload",
			workload: workload.NewDeploymentWorkload(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}),
			expectedKind: workload.KindDeployment,
		},
		{
			name: "daemonset workload",
			workload: workload.NewDaemonSetWorkload(&appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}),
			expectedKind: workload.KindDaemonSet,
		},
		{
			name: "statefulset workload",
			workload: workload.NewStatefulSetWorkload(&appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}),
			expectedKind: workload.KindStatefulSet,
		},
		{
			name: "job workload",
			workload: workload.NewJobWorkload(&batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}),
			expectedKind: workload.KindJob,
		},
		{
			name: "cronjob workload",
			workload: workload.NewCronJobWorkload(&batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			}),
			expectedKind: workload.KindCronJob,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the workload kind is correctly identified
			if tt.workload.Kind() != tt.expectedKind {
				t.Errorf("workload.Kind() = %v, want %v", tt.workload.Kind(), tt.expectedKind)
			}
		})
	}
}

func TestJobWorkloadTypeCast(t *testing.T) {
	// Test that JobWorkload type cast works correctly
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "test-job", Namespace: "default"},
	}
	jobWl := workload.NewJobWorkload(job)

	if jobWl.GetName() != "test-job" {
		t.Errorf("JobWorkload.GetName() = %v, want test-job", jobWl.GetName())
	}

	// Test GetJob method
	gotJob := jobWl.GetJob()
	if gotJob.Name != "test-job" {
		t.Errorf("JobWorkload.GetJob().Name = %v, want test-job", gotJob.Name)
	}

	// Verify it satisfies WorkloadAccessor interface
	var _ workload.WorkloadAccessor = jobWl
}

func TestCronJobWorkloadTypeCast(t *testing.T) {
	// Test that CronJobWorkload type cast works correctly
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cronjob", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
		},
	}
	cronJobWl := workload.NewCronJobWorkload(cronJob)

	if cronJobWl.GetName() != "test-cronjob" {
		t.Errorf("CronJobWorkload.GetName() = %v, want test-cronjob", cronJobWl.GetName())
	}

	// Test GetCronJob method
	gotCronJob := cronJobWl.GetCronJob()
	if gotCronJob.Name != "test-cronjob" {
		t.Errorf("CronJobWorkload.GetCronJob().Name = %v, want test-cronjob", gotCronJob.Name)
	}

	// Verify it satisfies WorkloadAccessor interface
	var _ workload.WorkloadAccessor = cronJobWl
}

func TestResourceTypeKind(t *testing.T) {
	// Test that ResourceType.Kind() returns correct values
	tests := []struct {
		resourceType reload.ResourceType
		expectedKind string
	}{
		{reload.ResourceTypeConfigMap, "ConfigMap"},
		{reload.ResourceTypeSecret, "Secret"},
	}

	for _, tt := range tests {
		t.Run(string(tt.resourceType), func(t *testing.T) {
			if got := tt.resourceType.Kind(); got != tt.expectedKind {
				t.Errorf("ResourceType.Kind() = %v, want %v", got, tt.expectedKind)
			}
		})
	}
}
