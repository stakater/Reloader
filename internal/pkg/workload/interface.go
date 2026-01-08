// Package workload provides an abstraction layer for Kubernetes workload types.
// It allows uniform handling of Deployments, DaemonSets, StatefulSets, Jobs, CronJobs, and Argo Rollouts.
//
// Note: Jobs and CronJobs have special update mechanisms:
// - Job: deleted and recreated with the same spec
// - CronJob: a new Job is created from the CronJob's template
package workload

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldManager is the field manager name used for server-side apply and patch operations.
// This identifies Reloader as the actor making changes to workload resources.
const FieldManager = "reloader"

// Kind represents the type of workload.
type Kind string

const (
	KindDeployment       Kind = "Deployment"
	KindDaemonSet        Kind = "DaemonSet"
	KindStatefulSet      Kind = "StatefulSet"
	KindArgoRollout      Kind = "Rollout"
	KindJob              Kind = "Job"
	KindCronJob          Kind = "CronJob"
	KindDeploymentConfig Kind = "DeploymentConfig"
)

// UpdateStrategy defines how a workload should be updated.
type UpdateStrategy int

const (
	// UpdateStrategyPatch uses strategic merge patch (default for most workloads).
	UpdateStrategyPatch UpdateStrategy = iota
	// UpdateStrategyRecreate deletes and recreates the workload (Jobs).
	UpdateStrategyRecreate
	// UpdateStrategyCreateNew creates a new resource from template (CronJobs).
	UpdateStrategyCreateNew
)

// WorkloadIdentity provides basic identification for a workload.
type WorkloadIdentity interface {
	// Kind returns the workload type.
	Kind() Kind

	// GetObject returns the underlying Kubernetes object.
	GetObject() client.Object

	// GetName returns the workload name.
	GetName() string

	// GetNamespace returns the workload namespace.
	GetNamespace() string
}

// WorkloadReader provides read-only access to workload state.
type WorkloadReader interface {
	WorkloadIdentity

	// GetAnnotations returns the workload's annotations.
	GetAnnotations() map[string]string

	// GetPodTemplateAnnotations returns annotations from the pod template spec.
	GetPodTemplateAnnotations() map[string]string

	// GetContainers returns all containers (including init containers).
	GetContainers() []corev1.Container

	// GetInitContainers returns all init containers.
	GetInitContainers() []corev1.Container

	// GetVolumes returns the pod template volumes.
	GetVolumes() []corev1.Volume

	// GetEnvFromSources returns all envFrom sources from all containers.
	GetEnvFromSources() []corev1.EnvFromSource

	// GetOwnerReferences returns the owner references of the workload.
	GetOwnerReferences() []metav1.OwnerReference
}

// WorkloadMatcher provides methods for checking resource usage.
type WorkloadMatcher interface {
	// UsesConfigMap checks if the workload uses a specific ConfigMap.
	UsesConfigMap(name string) bool

	// UsesSecret checks if the workload uses a specific Secret.
	UsesSecret(name string) bool
}

// WorkloadMutator provides methods for modifying workload state.
type WorkloadMutator interface {
	// SetPodTemplateAnnotation sets an annotation on the pod template.
	SetPodTemplateAnnotation(key, value string)

	// SetContainers updates the containers.
	SetContainers(containers []corev1.Container)

	// SetInitContainers updates the init containers.
	SetInitContainers(containers []corev1.Container)
}

// WorkloadUpdater provides methods for persisting workload changes.
type WorkloadUpdater interface {
	// Update persists changes to the workload.
	Update(ctx context.Context, c client.Client) error

	// UpdateStrategy returns how this workload should be updated.
	// Most workloads use UpdateStrategyPatch (strategic merge patch).
	// Jobs use UpdateStrategyRecreate (delete and recreate).
	// CronJobs use UpdateStrategyCreateNew (create a new Job from template).
	UpdateStrategy() UpdateStrategy

	// PerformSpecialUpdate handles non-standard update logic.
	// This is called when UpdateStrategy() != UpdateStrategyPatch.
	// For UpdateStrategyPatch workloads, this returns (false, nil).
	PerformSpecialUpdate(ctx context.Context, c client.Client) (updated bool, err error)

	// ResetOriginal resets the original state to the current object state.
	// This should be called after re-fetching the object (e.g., after a conflict)
	// to ensure strategic merge patch diffs are calculated correctly.
	ResetOriginal()

	// DeepCopy returns a deep copy of the workload.
	DeepCopy() Workload
}

// Workload combines all workload interfaces for full workload access.
// Use specific interfaces (WorkloadReader, WorkloadMatcher, etc.) when possible
// to limit scope and improve testability.
type Workload interface {
	WorkloadReader
	WorkloadMatcher
	WorkloadMutator
	WorkloadUpdater
}
