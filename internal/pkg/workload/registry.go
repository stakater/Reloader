package workload

import (
	"context"
	"fmt"
	"strings"

	argorolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	openshiftv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkloadLister is a function that lists workloads of a specific kind.
type WorkloadLister func(ctx context.Context, c client.Client, namespace string) ([]Workload, error)

// RegistryOptions configures the workload registry.
type RegistryOptions struct {
	ArgoRolloutsEnabled       bool
	DeploymentConfigEnabled   bool
	RolloutStrategyAnnotation string
}

// Registry provides factory methods for creating Workload instances.
type Registry struct {
	argoRolloutsEnabled       bool
	deploymentConfigEnabled   bool
	rolloutStrategyAnnotation string
	listers                   map[Kind]WorkloadLister
}

// NewRegistry creates a new workload registry.
func NewRegistry(opts RegistryOptions) *Registry {
	r := &Registry{
		argoRolloutsEnabled:       opts.ArgoRolloutsEnabled,
		deploymentConfigEnabled:   opts.DeploymentConfigEnabled,
		rolloutStrategyAnnotation: opts.RolloutStrategyAnnotation,
		listers: map[Kind]WorkloadLister{
			KindDeployment:  listDeployments,
			KindDaemonSet:   listDaemonSets,
			KindStatefulSet: listStatefulSets,
			KindJob:         listJobs,
			KindCronJob:     listCronJobs,
		},
	}
	if opts.ArgoRolloutsEnabled {
		// Use closure to capture the strategy annotation
		strategyAnnotation := opts.RolloutStrategyAnnotation
		r.listers[KindArgoRollout] = func(ctx context.Context, c client.Client, namespace string) ([]Workload, error) {
			var list argorolloutv1alpha1.RolloutList
			if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
				return nil, err
			}
			result := make([]Workload, len(list.Items))
			for i := range list.Items {
				result[i] = NewRolloutWorkload(&list.Items[i], strategyAnnotation)
			}
			return result, nil
		}
	}
	if opts.DeploymentConfigEnabled {
		r.listers[KindDeploymentConfig] = listDeploymentConfigs
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
	if r.deploymentConfigEnabled {
		kinds = append(kinds, KindDeploymentConfig)
	}
	return kinds
}

// FromObject creates a Workload from a Kubernetes object.
func (r *Registry) FromObject(obj client.Object) (Workload, error) {
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
		return NewRolloutWorkload(o, r.rolloutStrategyAnnotation), nil
	case *openshiftv1.DeploymentConfig:
		if !r.deploymentConfigEnabled {
			return nil, fmt.Errorf("openShift DeploymentConfig support is not enabled")
		}
		return NewDeploymentConfigWorkload(o), nil
	default:
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}
}

// kindAliases maps string representations to Kind constants.
// Supports lowercase, title case, and plural forms for user convenience.
var kindAliases = map[string]Kind{
	"deployment":        KindDeployment,
	"deployments":       KindDeployment,
	"daemonset":         KindDaemonSet,
	"daemonsets":        KindDaemonSet,
	"statefulset":       KindStatefulSet,
	"statefulsets":      KindStatefulSet,
	"rollout":           KindArgoRollout,
	"rollouts":          KindArgoRollout,
	"job":               KindJob,
	"jobs":              KindJob,
	"cronjob":           KindCronJob,
	"cronjobs":          KindCronJob,
	"deploymentconfig":  KindDeploymentConfig,
	"deploymentconfigs": KindDeploymentConfig,
}

// KindFromString converts a string to a Kind.
func KindFromString(s string) (Kind, error) {
	if k, ok := kindAliases[strings.ToLower(s)]; ok {
		return k, nil
	}
	return "", fmt.Errorf("unknown workload kind: %s", s)
}
