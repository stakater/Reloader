package workload

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Registry provides factory methods for creating Workload instances.
type Registry struct {
	argoRolloutsEnabled bool
}

// NewRegistry creates a new workload registry.
func NewRegistry(argoRolloutsEnabled bool) *Registry {
	return &Registry{
		argoRolloutsEnabled: argoRolloutsEnabled,
	}
}

// SupportedKinds returns all supported workload kinds.
func (r *Registry) SupportedKinds() []Kind {
	kinds := []Kind{
		KindDeployment,
		KindDaemonSet,
		KindStatefulSet,
		KindJob,
		KindCronJob,
	}
	if r.argoRolloutsEnabled {
		kinds = append(kinds, KindArgoRollout)
	}
	return kinds
}

// FromObject creates a WorkloadAccessor from a Kubernetes object.
func (r *Registry) FromObject(obj client.Object) (WorkloadAccessor, error) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return NewDeploymentWorkload(o), nil
	case *appsv1.DaemonSet:
		return NewDaemonSetWorkload(o), nil
	case *appsv1.StatefulSet:
		return NewStatefulSetWorkload(o), nil
	case *batchv1.Job:
		return NewJobWorkload(o), nil
	case *batchv1.CronJob:
		return NewCronJobWorkload(o), nil
	default:
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}
}

// KindFromString converts a string to a Kind.
func KindFromString(s string) (Kind, error) {
	switch s {
	case "Deployment", "deployment", "deployments":
		return KindDeployment, nil
	case "DaemonSet", "daemonset", "daemonsets":
		return KindDaemonSet, nil
	case "StatefulSet", "statefulset", "statefulsets":
		return KindStatefulSet, nil
	case "Rollout", "rollout", "rollouts":
		return KindArgoRollout, nil
	case "Job", "job", "jobs":
		return KindJob, nil
	case "CronJob", "cronjob", "cronjobs":
		return KindCronJob, nil
	default:
		return "", fmt.Errorf("unknown workload kind: %s", s)
	}
}
