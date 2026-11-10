package workload

import (
	"context"
	"strconv"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Progress reports how far a workload has got with a reload.
type Progress int

const (
	// ProgressNotApplied means the workload does not carry the reload yet.
	ProgressNotApplied Progress = iota
	// ProgressRunning means the reload was applied and is still rolling out or running.
	ProgressRunning
	// ProgressDone means the reload was applied and the workload settled successfully.
	ProgressDone
	// ProgressFailed means the reload was applied but the workload failed to settle.
	ProgressFailed
)

func (p Progress) String() string {
	switch p {
	case ProgressDone:
		return "done"
	case ProgressFailed:
		return "failed"
	case ProgressRunning:
		return "running"
	default:
		return "not-applied"
	}
}

// TemplateCheck reports whether a pod template carries a particular reload.
type TemplateCheck func(*corev1.PodTemplateSpec) bool

// progressDeadlineExceeded is the Progressing condition reason both Deployments
// and DeploymentConfigs report when a rollout stalls.
const progressDeadlineExceeded = "ProgressDeadlineExceeded"

// ReloadProgress is the fallback for kinds without rollout status: the reload is
// done as soon as the template carries it.
func (b *BaseWorkload[T]) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	if !applied(b.accessor.GetPodTemplateSpec()) {
		return ProgressNotApplied, nil
	}
	return ProgressDone, nil
}

// ReloadProgress reports done once every replica runs the reloaded template.
func (w *DeploymentWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	d := w.Object()
	if !applied(&d.Spec.Template) {
		return ProgressNotApplied, nil
	}
	if d.Status.ObservedGeneration < d.Generation {
		return ProgressRunning, nil
	}
	for _, cond := range d.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse &&
			cond.Reason == progressDeadlineExceeded {
			return ProgressFailed, nil
		}
	}
	replicas := replicasOrDefault(d.Spec.Replicas)
	if d.Status.UpdatedReplicas == replicas && d.Status.Replicas == replicas && d.Status.AvailableReplicas == replicas {
		return ProgressDone, nil
	}
	return ProgressRunning, nil
}

// ReloadProgress reports done once every replica runs the update revision.
// OnDelete StatefulSets never roll on their own, so they are done once observed.
func (w *StatefulSetWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	s := w.Object()
	if !applied(&s.Spec.Template) {
		return ProgressNotApplied, nil
	}
	if s.Status.ObservedGeneration < s.Generation {
		return ProgressRunning, nil
	}
	if s.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return ProgressDone, nil
	}
	replicas := replicasOrDefault(s.Spec.Replicas)
	if s.Status.UpdatedReplicas == replicas && s.Status.ReadyReplicas == replicas &&
		s.Status.CurrentRevision == s.Status.UpdateRevision {
		return ProgressDone, nil
	}
	return ProgressRunning, nil
}

// ReloadProgress reports done once every scheduled pod runs the reloaded template.
// OnDelete DaemonSets never roll on their own, so they are done once observed.
func (w *DaemonSetWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	ds := w.Object()
	if !applied(&ds.Spec.Template) {
		return ProgressNotApplied, nil
	}
	if ds.Status.ObservedGeneration < ds.Generation {
		return ProgressRunning, nil
	}
	if ds.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
		return ProgressDone, nil
	}
	desired := ds.Status.DesiredNumberScheduled
	if ds.Status.UpdatedNumberScheduled == desired && ds.Status.NumberAvailable == desired {
		return ProgressDone, nil
	}
	return ProgressRunning, nil
}

// ReloadProgress follows the Rollout phase once the controller observed the change.
func (w *RolloutWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	r := w.Object()
	if !applied(&r.Spec.Template) {
		return ProgressNotApplied, nil
	}
	if r.Status.ObservedGeneration != strconv.FormatInt(r.Generation, 10) {
		return ProgressRunning, nil
	}
	switch r.Status.Phase {
	case argorolloutv1alpha1.RolloutPhaseHealthy:
		return ProgressDone, nil
	case argorolloutv1alpha1.RolloutPhaseDegraded:
		return ProgressFailed, nil
	default:
		return ProgressRunning, nil
	}
}

// ReloadProgress reports done once every replica runs the latest version.
func (w *DeploymentConfigWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	dc := w.Object()
	if dc.Spec.Template == nil || !applied(dc.Spec.Template) {
		return ProgressNotApplied, nil
	}
	if dc.Status.ObservedGeneration < dc.Generation {
		return ProgressRunning, nil
	}
	for _, cond := range dc.Status.Conditions {
		if cond.Type == "Progressing" && cond.Status == corev1.ConditionFalse && cond.Reason == progressDeadlineExceeded {
			return ProgressFailed, nil
		}
	}
	replicas := dc.Spec.Replicas
	if dc.Status.UpdatedReplicas == replicas && dc.Status.AvailableReplicas == replicas && dc.Status.UnavailableReplicas == 0 {
		return ProgressDone, nil
	}
	return ProgressRunning, nil
}

// ReloadProgress follows the recreated Job to completion.
func (w *JobWorkload) ReloadProgress(_ context.Context, _ client.Reader, applied TemplateCheck) (Progress, error) {
	j := w.Object()
	if !applied(&j.Spec.Template) {
		return ProgressNotApplied, nil
	}
	return jobProgress(j), nil
}

// ReloadProgress follows the newest Job this CronJob owns that carries the reload.
func (w *CronJobWorkload) ReloadProgress(ctx context.Context, c client.Reader, applied TemplateCheck) (Progress, error) {
	cj := w.Object()
	var jobs batchv1.JobList
	if err := c.List(ctx, &jobs, client.InNamespace(cj.Namespace)); err != nil {
		return ProgressNotApplied, err
	}

	var newest *batchv1.Job
	for i := range jobs.Items {
		job := &jobs.Items[i]
		owner := metav1.GetControllerOf(job)
		if owner == nil || owner.UID != cj.UID || !applied(&job.Spec.Template) {
			continue
		}
		if newest == nil || newest.CreationTimestamp.Before(&job.CreationTimestamp) {
			newest = job
		}
	}
	if newest == nil {
		return ProgressNotApplied, nil
	}
	return jobProgress(newest), nil
}

func jobProgress(j *batchv1.Job) Progress {
	for _, cond := range j.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			continue
		}
		switch cond.Type {
		case batchv1.JobComplete:
			return ProgressDone
		case batchv1.JobFailed:
			return ProgressFailed
		}
	}
	return ProgressRunning
}

func replicasOrDefault(replicas *int32) int32 {
	if replicas == nil {
		return 1
	}
	return *replicas
}
