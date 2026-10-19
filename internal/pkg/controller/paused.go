package controller

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PausedRequeueInterval is how often a change held by the pause annotation is retried.
// The retry processes the resource's current state, so once the pause is lifted every
// change made in the meantime is applied without needing another edit.
const PausedRequeueInterval = 30 * time.Second

// pauseChecker reports whether reloads are paused for a resource, either through the
// pause annotation on the resource itself or on its namespace.
type pauseChecker struct {
	annotation string
	// reader is used to get Namespaces. It should be uncached so that Reloader does not
	// need to list and watch every namespace in the cluster.
	reader        client.Reader
	forbiddenOnce sync.Once
}

// pausedBy returns "resource" or "namespace" when reloads are paused, or "" when they are not.
// annotations may be nil for a deleted resource, in which case only the namespace is checked.
func (p *pauseChecker) pausedBy(ctx context.Context, log logr.Logger, namespace string, annotations map[string]string) (string, error) {
	if p.annotation == "" {
		return "", nil
	}
	if annotations[p.annotation] == "true" {
		return "resource", nil
	}

	var ns corev1.Namespace
	if err := p.reader.Get(ctx, client.ObjectKey{Name: namespace}, &ns); err != nil {
		switch {
		case errors.IsNotFound(err):
			return "", nil
		case errors.IsForbidden(err):
			// Namespaced installs are not granted access to Namespace objects. Keep
			// reloading rather than failing every reconcile, but say why the namespace
			// pause annotation has no effect.
			p.forbiddenOnce.Do(func() {
				log.Info("cannot get namespaces, the namespace pause annotation is ignored", "annotation", p.annotation, "error", err.Error())
			})
			return "", nil
		default:
			return "", err
		}
	}
	if ns.Annotations[p.annotation] == "true" {
		return "namespace", nil
	}
	return "", nil
}
