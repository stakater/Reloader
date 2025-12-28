package workload

import (
	"context"
	"fmt"
	"strings"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkloadLister is a function that lists workloads of a specific kind.
type WorkloadLister func(ctx context.Context, c client.Client, namespace string) ([]WorkloadAccessor, error)

// Registry provides factory methods for creating Workload instances.
type Registry struct {
	argoRolloutsEnabled bool
	listers             map[Kind]WorkloadLister
}

// NewRegistry creates a new workload registry.
func NewRegistry(argoRolloutsEnabled bool) *Registry {
	r := &Registry{
		argoRolloutsEnabled: argoRolloutsEnabled,
		listers: map[Kind]WorkloadLister{
			KindDeployment:  listDeployments,
			KindDaemonSet:   listDaemonSets,
			KindStatefulSet: listStatefulSets,
			KindJob:         listJobs,
			KindCronJob:     listCronJobs,
		},
	}
	if argoRolloutsEnabled {
		r.listers[KindArgoRollout] = listRollouts
	}
	return r
}

// ListerFor returns the lister function for the given kind, or nil if not found.
func (r *Registry) ListerFor(kind Kind) WorkloadLister {
	return r.listers[kind]
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
	case *argorolloutv1alpha1.Rollout:
		if !r.argoRolloutsEnabled {
			return nil, fmt.Errorf("argo Rollouts support is not enabled")
		}
		return NewRolloutWorkload(o), nil
	default:
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}
}

// kindAliases maps string representations to Kind constants.
// Supports lowercase, title case, and plural forms for user convenience.
var kindAliases = map[string]Kind{
	"deployment":  KindDeployment,
	"deployments": KindDeployment,
	"daemonset":   KindDaemonSet,
	"daemonsets":  KindDaemonSet,
	"statefulset": KindStatefulSet,
	"statefulsets": KindStatefulSet,
	"rollout":     KindArgoRollout,
	"rollouts":    KindArgoRollout,
	"job":         KindJob,
	"jobs":        KindJob,
	"cronjob":     KindCronJob,
	"cronjobs":    KindCronJob,
}

// KindFromString converts a string to a Kind.
func KindFromString(s string) (Kind, error) {
	if k, ok := kindAliases[strings.ToLower(s)]; ok {
		return k, nil
	}
	return "", fmt.Errorf("unknown workload kind: %s", s)
}
