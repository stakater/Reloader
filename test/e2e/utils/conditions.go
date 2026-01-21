package utils

import (
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	csiv1 "sigs.k8s.io/secrets-store-csi-driver/apis/v1"
)

// PodTemplateAccessor extracts PodTemplateSpec from a workload.
type PodTemplateAccessor[T any] func(T) *corev1.PodTemplateSpec

// AnnotationAccessor extracts annotations from a resource.
type AnnotationAccessor[T any] func(T) map[string]string

// ContainerAccessor extracts containers from a resource.
type ContainerAccessor[T any] func(T) []corev1.Container

// StatusAccessor extracts ready status from a resource.
type StatusAccessor[T any] func(T) bool

// UIDAccessor extracts UID from a resource.
type UIDAccessor[T any] func(T) types.UID

// ValueAccessor extracts a comparable value from a resource.
type ValueAccessor[T any, V comparable] func(T) V

// HasPodTemplateAnnotation returns a condition that checks for an annotation on the pod template.
func HasPodTemplateAnnotation[T any](accessor PodTemplateAccessor[T], key string) Condition[T] {
	return func(obj T) bool {
		template := accessor(obj)
		if template == nil || template.Annotations == nil {
			return false
		}
		_, ok := template.Annotations[key]
		return ok
	}
}

// HasAnnotation returns a condition that checks for an annotation on the resource.
func HasAnnotation[T any](accessor AnnotationAccessor[T], key string) Condition[T] {
	return func(obj T) bool {
		annotations := accessor(obj)
		if annotations == nil {
			return false
		}
		_, ok := annotations[key]
		return ok
	}
}

// NoAnnotation returns a condition that checks an annotation is absent.
func NoAnnotation[T any](accessor AnnotationAccessor[T], key string) Condition[T] {
	return func(obj T) bool {
		annotations := accessor(obj)
		if annotations == nil {
			return true
		}
		_, ok := annotations[key]
		return !ok
	}
}

// HasEnvVarPrefix returns a condition that checks for an env var with the given prefix.
func HasEnvVarPrefix[T any](accessor ContainerAccessor[T], prefix string) Condition[T] {
	return func(obj T) bool {
		containers := accessor(obj)
		for _, container := range containers {
			for _, env := range container.Env {
				if strings.HasPrefix(env.Name, prefix) {
					return true
				}
			}
		}
		return false
	}
}

// IsReady returns a condition that checks if the resource is ready.
func IsReady[T any](accessor StatusAccessor[T]) Condition[T] {
	return func(obj T) bool {
		return accessor(obj)
	}
}

// HasDifferentUID returns a condition that checks if the UID differs from original.
func HasDifferentUID[T any](accessor UIDAccessor[T], originalUID types.UID) Condition[T] {
	return func(obj T) bool {
		return accessor(obj) != originalUID
	}
}

// HasDifferentValue returns a condition that checks if a value differs from original.
func HasDifferentValue[T any, V comparable](accessor ValueAccessor[T, V], original V) Condition[T] {
	return func(obj T) bool {
		return accessor(obj) != original
	}
}

// And combines multiple conditions with AND logic.
func And[T any](conditions ...Condition[T]) Condition[T] {
	return func(obj T) bool {
		for _, cond := range conditions {
			if !cond(obj) {
				return false
			}
		}
		return true
	}
}

// Or combines multiple conditions with OR logic.
func Or[T any](conditions ...Condition[T]) Condition[T] {
	return func(obj T) bool {
		for _, cond := range conditions {
			if cond(obj) {
				return true
			}
		}
		return false
	}
}

// Always returns a condition that always returns true (for existence checks).
func Always[T any]() Condition[T] {
	return func(obj T) bool {
		return true
	}
}

// IsTriggeredJobForCronJob returns a condition that checks if a Job was triggered
// by Reloader for the specified CronJob (has owner reference and instantiate annotation).
func IsTriggeredJobForCronJob(cronJobName string) Condition[*batchv1.Job] {
	return func(job *batchv1.Job) bool {
		for _, ownerRef := range job.OwnerReferences {
			if ownerRef.Kind == "CronJob" && ownerRef.Name == cronJobName {
				if job.Annotations != nil {
					if _, ok := job.Annotations["cronjob.kubernetes.io/instantiate"]; ok {
						return true
					}
				}
			}
		}
		return false
	}
}

// SPCPSVersionChanged returns a condition that checks if the SPCPS version has changed
// from the initial version and the SPCPS is mounted.
func SPCPSVersionChanged(initialVersion string) Condition[*csiv1.SecretProviderClassPodStatus] {
	return func(spcps *csiv1.SecretProviderClassPodStatus) bool {
		if !spcps.Status.Mounted || len(spcps.Status.Objects) == 0 {
			return false
		}
		for _, obj := range spcps.Status.Objects {
			if obj.Version != initialVersion {
				return true
			}
		}
		return false
	}
}

// SPCPSForSPC returns a condition that checks if the SPCPS references a specific
// SecretProviderClass and is mounted.
func SPCPSForSPC(spcName string) Condition[*csiv1.SecretProviderClassPodStatus] {
	return func(spcps *csiv1.SecretProviderClassPodStatus) bool {
		return spcps.Status.SecretProviderClassName == spcName && spcps.Status.Mounted
	}
}

// SPCPSForPod returns a condition that checks if the SPCPS references a specific
// pod and is mounted.
func SPCPSForPod(podName string) Condition[*csiv1.SecretProviderClassPodStatus] {
	return func(spcps *csiv1.SecretProviderClassPodStatus) bool {
		return spcps.Status.PodName == podName && spcps.Status.Mounted
	}
}

// SPCPSForPods returns a condition that checks if the SPCPS references any of the
// specified pods and is mounted.
func SPCPSForPods(podNames map[string]bool) Condition[*csiv1.SecretProviderClassPodStatus] {
	return func(spcps *csiv1.SecretProviderClassPodStatus) bool {
		return podNames[spcps.Status.PodName] && spcps.Status.Mounted
	}
}
