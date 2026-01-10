package utils

import (
	"context"
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// AnnotationGetter retrieves annotations from a workload's pod template.
type AnnotationGetter func(ctx context.Context) (map[string]string, error)

// ContainerGetter retrieves containers from a workload's pod template.
type ContainerGetter func(ctx context.Context) ([]corev1.Container, error)

// WaitForAnnotation polls until an annotation key exists.
func WaitForAnnotation(ctx context.Context, getter AnnotationGetter, key string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		annotations, err := getter(ctx)
		if err != nil {
			return false, nil // Keep polling on errors
		}
		if annotations != nil {
			if _, ok := annotations[key]; ok {
				found = true
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return found, nil
}

// WaitForNoAnnotation polls until an annotation key is absent.
func WaitForNoAnnotation(ctx context.Context, getter AnnotationGetter, key string, timeout time.Duration) (bool, error) {
	var absent bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		annotations, err := getter(ctx)
		if err != nil {
			return false, nil
		}
		if annotations == nil {
			absent = true
			return true, nil
		}
		if _, ok := annotations[key]; !ok {
			absent = true
			return true, nil
		}
		return false, nil
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return absent, nil
}

// WaitForEnvVarPrefix polls until a container has an env var with given prefix.
func WaitForEnvVarPrefix(ctx context.Context, getter ContainerGetter, prefix string, timeout time.Duration) (bool, error) {
	var found bool
	err := wait.PollUntilContextTimeout(ctx, DefaultInterval, timeout, true, func(ctx context.Context) (bool, error) {
		containers, err := getter(ctx)
		if err != nil {
			return false, nil
		}
		for _, container := range containers {
			for _, env := range container.Env {
				if strings.HasPrefix(env.Name, prefix) {
					found = true
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return found, nil
}
