package workload

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IgnoreChecker checks if a workload kind should be ignored.
type IgnoreChecker interface {
	IsWorkloadIgnored(kind string) bool
}

// Lister lists workloads from the cluster.
type Lister struct {
	Client   client.Client
	Registry *Registry
	Checker  IgnoreChecker
}

// NewLister creates a new workload lister.
func NewLister(c client.Client, registry *Registry, checker IgnoreChecker) *Lister {
	return &Lister{
		Client:   c,
		Registry: registry,
		Checker:  checker,
	}
}

// List returns all workloads in the given namespace.
func (l *Lister) List(ctx context.Context, namespace string) ([]WorkloadAccessor, error) {
	var result []WorkloadAccessor

	for _, kind := range l.Registry.SupportedKinds() {
		if l.Checker != nil && l.Checker.IsWorkloadIgnored(string(kind)) {
			continue
		}

		workloads, err := l.listByKind(ctx, namespace, kind)
		if err != nil {
			return nil, err
		}
		result = append(result, workloads...)
	}

	return result, nil
}

func (l *Lister) listByKind(ctx context.Context, namespace string, kind Kind) ([]WorkloadAccessor, error) {
	switch kind {
	case KindDeployment:
		var list appsv1.DeploymentList
		if err := l.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		result := make([]WorkloadAccessor, len(list.Items))
		for i := range list.Items {
			result[i] = NewDeploymentWorkload(&list.Items[i])
		}
		return result, nil

	case KindDaemonSet:
		var list appsv1.DaemonSetList
		if err := l.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		result := make([]WorkloadAccessor, len(list.Items))
		for i := range list.Items {
			result[i] = NewDaemonSetWorkload(&list.Items[i])
		}
		return result, nil

	case KindStatefulSet:
		var list appsv1.StatefulSetList
		if err := l.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		result := make([]WorkloadAccessor, len(list.Items))
		for i := range list.Items {
			result[i] = NewStatefulSetWorkload(&list.Items[i])
		}
		return result, nil

	case KindJob:
		var list batchv1.JobList
		if err := l.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		result := make([]WorkloadAccessor, len(list.Items))
		for i := range list.Items {
			result[i] = NewJobWorkload(&list.Items[i])
		}
		return result, nil

	case KindCronJob:
		var list batchv1.CronJobList
		if err := l.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
			return nil, err
		}
		result := make([]WorkloadAccessor, len(list.Items))
		for i := range list.Items {
			result[i] = NewCronJobWorkload(&list.Items[i])
		}
		return result, nil

	default:
		return nil, nil
	}
}
