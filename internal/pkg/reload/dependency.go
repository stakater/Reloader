package reload

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/matcher"
)

// DependencyRef names a same-namespace workload listed in a reload-after annotation.
type DependencyRef struct {
	Kind workload.Kind `json:"kind"`
	Name string        `json:"name"`
}

func (r DependencyRef) String() string {
	return string(r.Kind) + "/" + r.Name
}

// ParseReloadAfter parses a comma-separated list of kind/name references.
func ParseReloadAfter(value string) ([]DependencyRef, error) {
	var refs []DependencyRef
	seen := make(map[DependencyRef]struct{})
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		kindStr, name, ok := strings.Cut(item, "/")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid reload-after entry %q, expected kind/name", item)
		}
		kind, err := workload.KindFromString(strings.TrimSpace(kindStr))
		if err != nil {
			return nil, fmt.Errorf("invalid reload-after entry %q: %w", item, err)
		}
		ref := DependencyRef{Kind: kind, Name: name}
		if _, dup := seen[ref]; dup {
			continue
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	return refs, nil
}

// PendingReload is a reload deferred until its dependencies finish reloading.
type PendingReload struct {
	ResourceType      matcher.ResourceType `json:"resourceType"`
	ResourceName      string               `json:"resourceName"`
	ResourceNamespace string               `json:"resourceNamespace"`
	Hash              string               `json:"hash"`
	AutoReload        bool                 `json:"autoReload,omitempty"`
	Dependencies      []DependencyRef      `json:"dependencies"`
	Since             time.Time            `json:"since"`
	Deadline          time.Time            `json:"deadline"`
}

// Key identifies the resource a pending reload belongs to; a newer change to the
// same resource replaces the older entry.
func (p PendingReload) Key() string {
	return string(p.ResourceType) + "/" + p.ResourceName
}

// PendingReloads maps PendingReload.Key to the entry. It is stored as JSON in the
// pending-reload annotation of the deferred workload.
type PendingReloads map[string]PendingReload

// ParsePendingReloads decodes the pending-reload annotation value.
func ParsePendingReloads(value string) (PendingReloads, error) {
	pending := PendingReloads{}
	if value == "" {
		return pending, nil
	}
	if err := json.Unmarshal([]byte(value), &pending); err != nil {
		return nil, fmt.Errorf("invalid pending-reload annotation: %w", err)
	}
	return pending, nil
}

// Encode returns the annotation value; empty when nothing is pending.
func (p PendingReloads) Encode() (string, error) {
	if len(p) == 0 {
		return "", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SortedKeys returns the entry keys in a stable order.
func (p PendingReloads) SortedKeys() []string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// PendingReloadAnnotation returns the annotation key holding deferred reloads.
func (s *Service) PendingReloadAnnotation() string {
	return s.cfg.Annotations.PendingReload
}

// ReloadAfter returns the parsed reload-after references of a workload.
func (s *Service) ReloadAfter(wl workload.Workload) ([]DependencyRef, error) {
	value := wl.GetAnnotations()[s.cfg.Annotations.ReloadAfter]
	if value == "" {
		return nil, nil
	}
	return ParseReloadAfter(value)
}

// ReloadAfterTimeout returns the workload's timeout override or the configured default.
func (s *Service) ReloadAfterTimeout(wl workload.Workload) time.Duration {
	value := wl.GetAnnotations()[s.cfg.Annotations.ReloadAfterTimeout]
	if value == "" {
		return s.cfg.ReloadAfterTimeout
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		s.reportAnnotationError(
			fmt.Errorf("invalid %s value %q, using default %s", s.cfg.Annotations.ReloadAfterTimeout, value, s.cfg.ReloadAfterTimeout),
			wl, wl.GetNamespace(),
		)
		return s.cfg.ReloadAfterTimeout
	}
	return d
}

// ReportReloadAfterError logs an unusable reload-after annotation once.
func (s *Service) ReportReloadAfterError(err error, wl workload.Workload) {
	s.reportAnnotationError(err, wl, wl.GetNamespace())
}

// AppliedCheck returns a TemplateCheck that reports whether a pod template carries
// the reload of the given resource at the given hash, under either strategy.
func (s *Service) AppliedCheck(resourceName string, resourceType matcher.ResourceType, hash string) workload.TemplateCheck {
	envName := NewEnvVarStrategy().envVarName(resourceName, resourceType)
	annotationKey := s.cfg.Annotations.LastReloadedFrom
	return func(tmpl *corev1.PodTemplateSpec) bool {
		if tmpl == nil {
			return false
		}
		if value, ok := tmpl.Annotations[annotationKey]; ok {
			var src ReloadSource
			if err := json.Unmarshal([]byte(value), &src); err == nil &&
				src.Kind == string(resourceType) && src.Name == resourceName && src.Hash == hash {
				return true
			}
		}
		for _, containers := range [][]corev1.Container{tmpl.Spec.Containers, tmpl.Spec.InitContainers} {
			for _, c := range containers {
				for _, env := range c.Env {
					if env.Name == envName && env.Value == hash {
						return true
					}
				}
			}
		}
		return false
	}
}
