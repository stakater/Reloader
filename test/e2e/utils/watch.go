package utils

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Timeout constants for watch operations.
const (
	DefaultInterval      = 1 * time.Second  // Polling interval
	ShortTimeout         = 5 * time.Second  // Quick checks
	NegativeTestWait     = 3 * time.Second  // Wait before checking negative conditions
	WorkloadReadyTimeout = 60 * time.Second // Workload readiness timeout (buffer for CI)
	ReloadTimeout        = 15 * time.Second // Time for reload to trigger
)

// ErrWatchTimeout is returned when a watch times out waiting for condition.
var ErrWatchTimeout = errors.New("watch timeout waiting for condition")

// ErrUnsupportedOperation is returned when an operation is not supported for a workload type.
var ErrUnsupportedOperation = errors.New("operation not supported for this workload type")

// HandleWatchResult converts watch errors to the standard (bool, error) return pattern.
// Returns (false, nil) for timeout, (true, nil) for success, (false, err) for other errors.
func HandleWatchResult(err error) (bool, error) {
	if errors.Is(err, ErrWatchTimeout) {
		return false, nil
	}
	return err == nil, err
}

// WatchFunc is a function that starts a watch for a specific resource.
type WatchFunc func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

// Condition is a function that checks if the desired state is reached.
type Condition[T any] func(T) bool

// WatchUntil watches a resource until the condition is met or timeout occurs.
// It handles watch reconnection automatically on errors.
// If name is empty, it watches all resources and returns the first matching one.
func WatchUntil[T runtime.Object](ctx context.Context, watchFunc WatchFunc, name string, condition Condition[T], timeout time.Duration) (T, error) {
	var zero T
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := metav1.ListOptions{Watch: true}
	if name != "" {
		opts.FieldSelector = fields.OneTermEqualSelector("metadata.name", name).String()
	}

	for {
		select {
		case <-ctx.Done():
			return zero, ErrWatchTimeout
		default:
		}

		result, done, err := watchOnce(ctx, watchFunc, opts, condition)
		if done {
			return result, err
		}
		select {
		case <-ctx.Done():
			return zero, ErrWatchTimeout
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// watchOnce starts a single watch and processes events until condition met or watch ends.
func watchOnce[T runtime.Object](
	ctx context.Context,
	watchFunc WatchFunc,
	opts metav1.ListOptions,
	condition Condition[T],
) (T, bool, error) {
	var zero T

	watcher, err := watchFunc(ctx, opts)
	if err != nil {
		return zero, false, nil
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return zero, true, ErrWatchTimeout
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return zero, false, nil
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				obj, ok := event.Object.(T)
				if !ok {
					continue
				}
				if condition(obj) {
					return obj, true, nil
				}
			case watch.Deleted:
				continue
			case watch.Error:
				return zero, false, nil
			}
		}
	}
}

// WatchUntilDeleted watches until the resource is deleted or timeout occurs.
func WatchUntilDeleted(
	ctx context.Context,
	watchFunc WatchFunc,
	name string,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
		Watch:         true,
	}

	for {
		select {
		case <-ctx.Done():
			return ErrWatchTimeout
		default:
		}

		deleted, err := watchDeleteOnce(ctx, watchFunc, opts)
		if deleted {
			return err
		}
		select {
		case <-ctx.Done():
			return ErrWatchTimeout
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func watchDeleteOnce(
	ctx context.Context,
	watchFunc WatchFunc,
	opts metav1.ListOptions,
) (bool, error) {
	watcher, err := watchFunc(ctx, opts)
	if err != nil {
		return false, nil
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return true, ErrWatchTimeout
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return false, nil
			}
			if event.Type == watch.Deleted {
				return true, nil
			}
			if event.Type == watch.Error {
				return false, nil
			}
		}
	}
}

// WatchUntilDifferentUID watches until the resource has a different UID (recreated).
func WatchUntilDifferentUID[T runtime.Object](
	ctx context.Context,
	watchFunc WatchFunc,
	name string,
	originalUID string,
	timeout time.Duration,
	getUID func(T) string,
) (T, bool, error) {
	var zero T
	result, err := WatchUntil(ctx, watchFunc, name, func(obj T) bool {
		return getUID(obj) != originalUID
	}, timeout)
	if errors.Is(err, ErrWatchTimeout) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, err
	}
	return result, true, nil
}
