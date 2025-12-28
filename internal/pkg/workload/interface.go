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

// Kind represents the type of workload.
type Kind string

const (
	KindDeployment  Kind = "Deployment"
	KindDaemonSet   Kind = "DaemonSet"
	KindStatefulSet Kind = "StatefulSet"
	KindArgoRollout Kind = "Rollout"
	KindJob         Kind = "Job"
	KindCronJob     Kind = "CronJob"
)

// Workload provides a uniform interface for managing Kubernetes workloads.
// All implementations must be safe for concurrent use.
type Workload interface {
	// Kind returns the workload type.
	Kind() Kind

	// GetObject returns the underlying Kubernetes object.
	GetObject() client.Object

	// GetName returns the workload name.
	GetName() string

	// GetNamespace returns the workload namespace.
	GetNamespace() string

	// GetAnnotations returns the workload's annotations.
	GetAnnotations() map[string]string

	// GetPodTemplateAnnotations returns annotations from the pod template spec.
	GetPodTemplateAnnotations() map[string]string

	// SetPodTemplateAnnotation sets an annotation on the pod template.
	SetPodTemplateAnnotation(key, value string)

	// GetContainers returns all containers (including init containers).
	GetContainers() []corev1.Container

	// SetContainers updates the containers.
	SetContainers(containers []corev1.Container)

	// GetInitContainers returns all init containers.
	GetInitContainers() []corev1.Container

	// SetInitContainers updates the init containers.
	SetInitContainers(containers []corev1.Container)

	// GetVolumes returns the pod template volumes.
	GetVolumes() []corev1.Volume

	// Update persists changes to the workload.
	Update(ctx context.Context, c client.Client) error

	// DeepCopy returns a deep copy of the workload.
	DeepCopy() Workload
}

// Accessor provides read-only access to workload configuration.
// Use this interface when you only need to inspect workload state.
type Accessor interface {
	// Kind returns the workload type.
	Kind() Kind

	// GetName returns the workload name.
	GetName() string

	// GetNamespace returns the workload namespace.
	GetNamespace() string

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

	// UsesConfigMap checks if the workload uses a specific ConfigMap.
	UsesConfigMap(name string) bool

	// UsesSecret checks if the workload uses a specific Secret.
	UsesSecret(name string) bool

	// GetOwnerReferences returns the owner references of the workload.
	GetOwnerReferences() []metav1.OwnerReference
}

// WorkloadAccessor provides both Workload and Accessor interfaces.
// This is the primary type returned by the registry.
type WorkloadAccessor interface {
	Workload
	Accessor
}
