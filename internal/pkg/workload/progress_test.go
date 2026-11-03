package workload

import (
	"context"
	"testing"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func appliedAlways(*corev1.PodTemplateSpec) bool { return true }
func appliedNever(*corev1.PodTemplateSpec) bool  { return false }

func TestDeploymentReloadProgress(t *testing.T) {
	two := int32(2)
	base := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 3},
			Spec:       appsv1.DeploymentSpec{Replicas: &two},
			Status: appsv1.DeploymentStatus{
				ObservedGeneration: 3, Replicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2,
			},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*appsv1.Deployment)
		applied TemplateCheck
		want    Progress
	}{
		{name: "not applied", applied: appliedNever, want: ProgressNotApplied},
		{name: "rolled out", want: ProgressDone},
		{name: "not observed", mutate: func(d *appsv1.Deployment) { d.Status.ObservedGeneration = 2 }, want: ProgressRunning},
		{name: "old pods remain", mutate: func(d *appsv1.Deployment) { d.Status.Replicas = 3 }, want: ProgressRunning},
		{name: "not available", mutate: func(d *appsv1.Deployment) { d.Status.AvailableReplicas = 1 }, want: ProgressRunning},
		{
			name: "deadline exceeded",
			mutate: func(d *appsv1.Deployment) {
				d.Status.UpdatedReplicas = 1
				d.Status.Conditions = []appsv1.DeploymentCondition{{
					Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
				}}
			},
			want: ProgressFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := base()
			if tt.mutate != nil {
				tt.mutate(d)
			}
			applied := tt.applied
			if applied == nil {
				applied = appliedAlways
			}
			got, err := NewDeploymentWorkload(d).ReloadProgress(context.Background(), nil, applied)
			if err != nil || got != tt.want {
				t.Errorf("ReloadProgress() = %v, %v; want %v", got, err, tt.want)
			}
		})
	}
}

func TestStatefulSetAndDaemonSetReloadProgress(t *testing.T) {
	one := int32(1)
	sts := &appsv1.StatefulSet{
		Spec:   appsv1.StatefulSetSpec{Replicas: &one},
		Status: appsv1.StatefulSetStatus{UpdatedReplicas: 1, ReadyReplicas: 1, CurrentRevision: "a", UpdateRevision: "b"},
	}
	if got, _ := NewStatefulSetWorkload(sts).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressRunning {
		t.Errorf("statefulset mid-rollout = %v, want running", got)
	}
	sts.Status.CurrentRevision = "b"
	if got, _ := NewStatefulSetWorkload(sts).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressDone {
		t.Errorf("statefulset rolled out = %v, want done", got)
	}

	ds := &appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, UpdatedNumberScheduled: 2, NumberAvailable: 3}}
	if got, _ := NewDaemonSetWorkload(ds).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressRunning {
		t.Errorf("daemonset mid-rollout = %v, want running", got)
	}
	ds.Spec.UpdateStrategy.Type = appsv1.OnDeleteDaemonSetStrategyType
	if got, _ := NewDaemonSetWorkload(ds).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressDone {
		t.Errorf("OnDelete daemonset = %v, want done", got)
	}
}

func TestRolloutReloadProgress(t *testing.T) {
	r := &argorolloutv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Generation: 2}}
	r.Status.ObservedGeneration = "1"
	r.Status.Phase = argorolloutv1alpha1.RolloutPhaseHealthy
	if got, _ := NewRolloutWorkload(r, "").ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressRunning {
		t.Errorf("unobserved rollout = %v, want running", got)
	}
	r.Status.ObservedGeneration = "2"
	if got, _ := NewRolloutWorkload(r, "").ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressDone {
		t.Errorf("healthy rollout = %v, want done", got)
	}
	r.Status.Phase = argorolloutv1alpha1.RolloutPhaseDegraded
	if got, _ := NewRolloutWorkload(r, "").ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressFailed {
		t.Errorf("degraded rollout = %v, want failed", got)
	}
}

func TestJobReloadProgress(t *testing.T) {
	job := &batchv1.Job{}
	if got, _ := NewJobWorkload(job).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressRunning {
		t.Errorf("running job = %v, want running", got)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	if got, _ := NewJobWorkload(job).ReloadProgress(context.Background(), nil, appliedAlways); got != ProgressFailed {
		t.Errorf("failed job = %v, want failed", got)
	}
}

func TestCronJobReloadProgress(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)

	cj := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "migrate", Namespace: "ns", UID: "cj-uid"}}
	owned := func(name string, created int64, marked bool, cond batchv1.JobConditionType) *batchv1.Job {
		j := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: "ns",
			CreationTimestamp: metav1.Unix(created, 0),
			OwnerReferences:   []metav1.OwnerReference{*metav1.NewControllerRef(cj, batchv1.SchemeGroupVersion.WithKind("CronJob"))},
		}}
		if marked {
			j.Spec.Template.Annotations = map[string]string{"marked": "true"}
		}
		if cond != "" {
			j.Status.Conditions = []batchv1.JobCondition{{Type: cond, Status: corev1.ConditionTrue}}
		}
		return j
	}
	marked := func(tmpl *corev1.PodTemplateSpec) bool { return tmpl.Annotations["marked"] == "true" }

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		owned("scheduled", 100, false, batchv1.JobComplete),
		owned("old-reload", 200, true, batchv1.JobFailed),
		owned("new-reload", 300, true, batchv1.JobComplete),
	).Build()

	got, err := NewCronJobWorkload(cj).ReloadProgress(context.Background(), c, marked)
	if err != nil || got != ProgressDone {
		t.Errorf("ReloadProgress() = %v, %v; want done from the newest marked job", got, err)
	}

	empty := fake.NewClientBuilder().WithScheme(scheme).WithObjects(owned("scheduled", 100, false, batchv1.JobComplete)).Build()
	if got, _ := NewCronJobWorkload(cj).ReloadProgress(context.Background(), empty, marked); got != ProgressNotApplied {
		t.Errorf("no marked job = %v, want not-applied", got)
	}
}
