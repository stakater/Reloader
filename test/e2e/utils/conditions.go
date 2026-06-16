package utils

import (
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

// HasPodTemplateAnnotationChanged returns a condition that checks the pod template annotation
// is present AND its value differs from priorValue. If priorValue is empty, any non-empty value
// satisfies the condition (equivalent to HasPodTemplateAnnotation).
// Use this in WaitReloaded to correctly detect a reload after a prior reload has already set the annotation.
func HasPodTemplateAnnotationChanged[T any](accessor PodTemplateAccessor[T], key, priorValue string) Condition[T] {
	return func(obj T) bool {
		template := accessor(obj)
		if template == nil || template.Annotations == nil {
			return false
		}
		v, ok := template.Annotations[key]
		if !ok {
			return false
		}
		if priorValue == "" {
			return true
		}
		return v != priorValue
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

// HasEnvVarNamed returns a condition that checks for an env var with exactly the given name.
func HasEnvVarNamed[T any](accessor ContainerAccessor[T], name string) Condition[T] {
	return func(obj T) bool {
		containers := accessor(obj)
		for _, container := range containers {
			for _, env := range container.Env {
				if env.Name == name {
					return true
				}
			}
		}
		return false
	}
}

// HasEnvVarPrefixChanged returns a condition that checks for an env var with the given prefix
// whose value has changed from priorValue. If priorValue is empty, any matching env var satisfies
// the condition (equivalent to HasEnvVarPrefix).
// Use this in WaitEnvVar to correctly detect a reload after a prior reload already set the env var.
func HasEnvVarPrefixChanged[T any](accessor ContainerAccessor[T], prefix, priorValue string) Condition[T] {
	return func(obj T) bool {
		containers := accessor(obj)
		for _, container := range containers {
			for _, env := range container.Env {
				if strings.HasPrefix(env.Name, prefix) {
					if priorValue == "" {
						return true
					}
					return env.Value != priorValue
				}
			}
		}
		return false
	}
}

// GetEnvVarValueByPrefix returns the value of the first env var with the given prefix
// found across the given containers. Returns empty string if not found.
func GetEnvVarValueByPrefix(containers []corev1.Container, prefix string) string {
	for _, c := range containers {
		for _, env := range c.Env {
			if strings.HasPrefix(env.Name, prefix) {
				return env.Value
			}
		}
	}
	return ""
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
