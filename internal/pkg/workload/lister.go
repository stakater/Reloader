package workload

import (
	"context"

	openshiftv1 "github.com/openshift/api/apps/v1"
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
func (l *Lister) List(ctx context.Context, namespace string) ([]Workload, error) {
	var result []Workload

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

func (l *Lister) listByKind(ctx context.Context, namespace string, kind Kind) ([]Workload, error) {
	lister := l.Registry.ListerFor(kind)
	if lister == nil {
		return nil, nil
	}
	return lister(ctx, l.Client, namespace)
}

func listDeployments(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list appsv1.DeploymentList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewDeploymentWorkload(&list.Items[i])
	}
	return result, nil
}

func listDaemonSets(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list appsv1.DaemonSetList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewDaemonSetWorkload(&list.Items[i])
	}
	return result, nil
}

func listStatefulSets(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list appsv1.StatefulSetList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewStatefulSetWorkload(&list.Items[i])
	}
	return result, nil
}

func listJobs(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list batchv1.JobList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewJobWorkload(&list.Items[i])
	}
	return result, nil
}

func listCronJobs(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list batchv1.CronJobList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewCronJobWorkload(&list.Items[i])
	}
	return result, nil
}

func listDeploymentConfigs(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
	var list openshiftv1.DeploymentConfigList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	result := make([]Workload, len(list.Items))
	for i := range list.Items {
		result[i] = NewDeploymentConfigWorkload(&list.Items[i])
	}
	return result, nil
}
