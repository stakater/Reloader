package reload

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
	"github.com/stakater/Reloader/pkg/matcher"
)

// Service orchestrates the reload logic for ConfigMaps and Secrets.
// External, decision-only consumers should use the public matcher package instead.
type Service struct {
	cfg      *config.Config
	log      logr.Logger
	hasher   *Hasher
	matcher  *matcher.Matcher
	strategy Strategy
	// now is the clock sync windows are evaluated against.
	now func() time.Time
	// reportedAnnotationErrors remembers annotation problems already logged. The
	// annotations are static while this runs once per workload per event, so without
	// it one typo on a templated workload floods the log for the process lifetime.
	reportedAnnotationErrors sync.Map
}

// NewService creates a new reload Service with the given configuration.
func NewService(cfg *config.Config, log logr.Logger) *Service {
	return &Service{
		cfg:      cfg,
		log:      log,
		hasher:   NewHasher(),
		matcher:  matcher.NewMatcher(cfg),
		strategy: NewStrategy(cfg),
		now:      time.Now,
	}
}

// Process evaluates all workloads to determine which should be reloaded.
func (s *Service) Process(change ResourceChange, workloads []workload.Workload) []ReloadDecision {
	if change.IsNil() {
		return nil
	}

	if !s.shouldProcessEvent(change.GetEventType()) {
		return nil
	}

	hash := change.ComputeHash(s.hasher)
	if change.GetEventType() == EventTypeDelete {
		hash = s.hasher.EmptyHash()
	}

	return s.processResource(
		change.GetName(),
		change.GetNamespace(),
		change.GetAnnotations(),
		change.GetResourceType(),
		hash,
		workloads,
	)
}

func (s *Service) processResource(
	resourceName string,
	resourceNamespace string,
	resourceAnnotations map[string]string,
	resourceType matcher.ResourceType,
	hash string,
	workloads []workload.Workload,
) []ReloadDecision {
	var decisions []ReloadDecision

	for _, wl := range workloads {
		if wl.GetNamespace() != resourceNamespace {
			continue
		}

		if s.cfg.IsWorkloadIgnored(string(wl.Kind())) {
			continue
		}

		var usesResource bool
		switch resourceType {
		case matcher.ResourceTypeConfigMap:
			usesResource = wl.UsesConfigMap(resourceName)
		case matcher.ResourceTypeSecret:
			usesResource = wl.UsesSecret(resourceName)
		case matcher.ResourceTypeSecretProviderClass:
			// Annotation-only matching (parity with master): the workload's
			// annotations alone decide the reload; no volume-uses scan.
			usesResource = true
		}

		input := matcher.MatchInput{
			ResourceName:        resourceName,
			ResourceNamespace:   resourceNamespace,
			ResourceType:        resourceType,
			ResourceAnnotations: resourceAnnotations,
			WorkloadAnnotations: wl.GetAnnotations(),
			PodAnnotations:      wl.GetPodTemplateAnnotations(),
		}

		matchResult := s.matcher.ShouldReload(input)

		for _, err := range matchResult.Errors {
			s.reportAnnotationError(err, wl, resourceNamespace)
		}

		shouldReload := matchResult.ShouldReload
		if matchResult.AutoReload && !usesResource {
			shouldReload = false
		}

		var deferUntil time.Time
		if shouldReload {
			var err error
			if deferUntil, err = s.syncWindowDeferral(wl, resourceType, resourceName, hash); err != nil {
				s.reportAnnotationError(err, wl, resourceNamespace)
			}
		}

		decisions = append(
			decisions, ReloadDecision{
				Workload:     wl,
				ShouldReload: shouldReload,
				AutoReload:   matchResult.AutoReload,
				Reason:       matchResult.Reason,
				Hash:         hash,
				DeferUntil:   deferUntil,
			},
		)
	}

	return decisions
}

// syncWindowDeferral returns when the workload's sync windows let it reload for this
// change, or the zero time when it may reload now. The annotation is read from the
// workload, then from its pod template. An unusable annotation is reported as an error and
// the workload reloads as if it had none, matching how an unusable pause-period annotation
// is handled.
//
// A reload already deferred for the same content keeps the time recorded in the
// pending-reload annotation, so a restart neither moves it nor lets it start early.
// Otherwise, outside the windows, the reload is due when they next allow it, plus an offset
// that spreads the deferred workloads over the start of the window.
func (s *Service) syncWindowDeferral(
	wl workload.Workload,
	resourceType matcher.ResourceType,
	resourceName string,
	hash string,
) (time.Time, error) {
	key := s.cfg.Annotations.SyncWindows
	// A deferral is recorded in the pending-reload annotation, so without its key there is
	// nowhere to keep one.
	if key == "" || s.cfg.Annotations.PendingReload == "" {
		return time.Time{}, nil
	}

	value := wl.GetAnnotations()[key]
	if value == "" {
		value = wl.GetPodTemplateAnnotations()[key]
	}
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	if !SupportsSyncWindows(wl.Kind()) {
		return time.Time{}, fmt.Errorf("ignoring %s annotation on %s, which does not support sync windows", key, wl.Kind())
	}

	windows, err := ParseSyncWindows(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("ignoring %s annotation, reloading without sync windows: %w", key, err)
	}

	now := s.now()
	// Parsing rejects schedules that never fire, so while blocked there is always a later
	// time to look again at, even when the search ends before finding an allowed one.
	opens, _ := windows.NextAllowed(now)
	offset, spread := s.spreadOffset(wl, windows, opens)

	// A reload already deferred for this content keeps its recorded time, but only while
	// that time is one the current windows could have produced: a hand-edited or stale
	// annotation must not postpone the reload past the window.
	pending, ok := FindPendingReload(wl.GetAnnotations(), s.cfg.Annotations.PendingReload, resourceType, resourceName)
	if ok && pending.Hash == hash && now.Before(pending.NotBefore) && !pending.NotBefore.After(opens.Add(spread)) {
		return pending.NotBefore, nil
	}
	if opens.Equal(now) {
		return time.Time{}, nil
	}
	return opens.Add(offset), nil
}

// SupportsSyncWindows reports whether sync windows can defer reloads of a workload kind.
// A Job or CronJob reload starts a new run rather than restarting long-lived pods, and a
// CronJob's next scheduled run reads the new configuration anyway, so neither is deferred.
// Reloader also has no update permission on Jobs to record a deferral on them.
func SupportsSyncWindows(kind workload.Kind) bool {
	switch kind {
	case workload.KindJob, workload.KindCronJob:
		return false
	default:
		return true
	}
}

// spreadOffset returns how long after the window opens the workload reloads, and the
// spread the offset was drawn from. The offset is derived from the workload's identity, so
// it is stable across restarts and spreads many workloads evenly. The spread is kept within
// the first half of the allowed period, so the reload still starts before the window closes.
func (s *Service) spreadOffset(wl workload.Workload, windows SyncWindows, opens time.Time) (time.Duration, time.Duration) {
	spread := windows.Spread(s.cfg.SyncWindowSpread)
	if closes, ok := windows.NextBlocked(opens); ok {
		spread = min(spread, closes.Sub(opens)/2)
	}
	spread = spread.Truncate(time.Second)
	if spread <= 0 {
		return 0, 0
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(string(wl.Kind()) + "/" + wl.GetNamespace() + "/" + wl.GetName()))
	seconds := h.Sum64() % uint64(spread/time.Second)
	return time.Duration(seconds) * time.Second, spread
}

func (s *Service) shouldProcessEvent(eventType EventType) bool {
	switch eventType {
	case EventTypeCreate:
		return s.cfg.ReloadOnCreate
	case EventTypeDelete:
		return s.cfg.ReloadOnDelete
	case EventTypeUpdate:
		return true
	default:
		return false
	}
}

// ApplyReload applies the reload strategy to a workload.
func (s *Service) ApplyReload(
	ctx context.Context,
	wl workload.Workload,
	resourceName string,
	resourceType matcher.ResourceType,
	namespace string,
	hash string,
	autoReload bool,
) (bool, error) {
	container := s.findTargetContainer(wl, resourceName, resourceType, autoReload)

	input := StrategyInput{
		ResourceName:   resourceName,
		ResourceType:   resourceType,
		Namespace:      namespace,
		Hash:           hash,
		Container:      container,
		PodAnnotations: wl.GetPodTemplateAnnotations(),
		AutoReload:     autoReload,
	}

	updated, err := s.strategy.Apply(input)
	if err != nil {
		return false, err
	}

	if updated {
		// The reload this change was waiting for is happening, so it is no longer pending.
		// The same write persists both.
		ClearPendingReload(wl.GetObject(), s.cfg.Annotations.PendingReload, resourceType, resourceName)

		// Attribution annotation is informational; log errors but don't fail reloads
		if err := s.setAttributionAnnotation(wl, resourceName, resourceType, namespace, hash, container); err != nil {
			s.log.V(1).Info("failed to set attribution annotation", "error", err, "workload", wl.GetName())
		}
	}

	return updated, nil
}

func (s *Service) setAttributionAnnotation(
	wl workload.Workload,
	resourceName string,
	resourceType matcher.ResourceType,
	namespace string,
	hash string,
	container *corev1.Container,
) error {
	containerName := ""
	if container != nil {
		containerName = container.Name
	}

	source := ReloadSource{
		Kind:       string(resourceType),
		Name:       resourceName,
		Namespace:  namespace,
		Hash:       hash,
		Containers: []string{containerName},
		ReloadedAt: time.Now().UTC(),
	}

	sourceJSON, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("failed to marshal reload source: %w", err)
	}

	wl.SetPodTemplateAnnotation(s.cfg.Annotations.LastReloadedFrom, string(sourceJSON))
	return nil
}

func (s *Service) findTargetContainer(
	wl workload.Workload,
	resourceName string,
	resourceType matcher.ResourceType,
	autoReload bool,
) *corev1.Container {
	containers := wl.GetContainers()
	if len(containers) == 0 {
		return nil
	}

	if !autoReload {
		return &containers[0]
	}

	volumes := wl.GetVolumes()
	initContainers := wl.GetInitContainers()

	volumeName := s.findVolumeUsingResource(volumes, resourceName, resourceType)
	if volumeName != "" {
		container := s.findContainerWithVolumeMount(containers, volumeName)
		if container != nil {
			return container
		}
		container = s.findContainerWithVolumeMount(initContainers, volumeName)
		if container != nil {
			return &containers[0]
		}
	}

	container := s.findContainerWithEnvRef(containers, resourceName, resourceType)
	if container != nil {
		return container
	}

	container = s.findContainerWithEnvRef(initContainers, resourceName, resourceType)
	if container != nil {
		return &containers[0]
	}

	return &containers[0]
}

func (s *Service) findVolumeUsingResource(volumes []corev1.Volume, resourceName string, resourceType matcher.ResourceType) string {
	for _, vol := range volumes {
		switch resourceType {
		case matcher.ResourceTypeConfigMap:
			if vol.ConfigMap != nil && vol.ConfigMap.Name == resourceName {
				return vol.Name
			}
			if vol.Projected != nil {
				for _, src := range vol.Projected.Sources {
					if src.ConfigMap != nil && src.ConfigMap.Name == resourceName {
						return vol.Name
					}
				}
			}
		case matcher.ResourceTypeSecret:
			if vol.Secret != nil && vol.Secret.SecretName == resourceName {
				return vol.Name
			}
			if vol.Projected != nil {
				for _, src := range vol.Projected.Sources {
					if src.Secret != nil && src.Secret.Name == resourceName {
						return vol.Name
					}
				}
			}
		case matcher.ResourceTypeSecretProviderClass:
			// Match the CSI volume that references this SPC.
			if vol.CSI != nil && vol.CSI.VolumeAttributes["secretProviderClass"] == resourceName {
				return vol.Name
			}
		}
	}
	return ""
}

func (s *Service) findContainerWithVolumeMount(containers []corev1.Container, volumeName string) *corev1.Container {
	for i := range containers {
		for _, mount := range containers[i].VolumeMounts {
			if mount.Name == volumeName {
				return &containers[i]
			}
		}
	}
	return nil
}

func (s *Service) findContainerWithEnvRef(containers []corev1.Container, resourceName string, resourceType matcher.ResourceType) *corev1.Container {
	for i := range containers {
		for _, env := range containers[i].Env {
			if env.ValueFrom == nil {
				continue
			}
			switch resourceType {
			case matcher.ResourceTypeConfigMap:
				if env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == resourceName {
					return &containers[i]
				}
			case matcher.ResourceTypeSecret:
				if env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == resourceName {
					return &containers[i]
				}
			}
		}

		for _, envFrom := range containers[i].EnvFrom {
			switch resourceType {
			case matcher.ResourceTypeConfigMap:
				if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == resourceName {
					return &containers[i]
				}
			case matcher.ResourceTypeSecret:
				if envFrom.SecretRef != nil && envFrom.SecretRef.Name == resourceName {
					return &containers[i]
				}
			}
		}
	}
	return nil
}

// Hasher returns the hasher used by this service.
func (s *Service) Hasher() *Hasher {
	return s.hasher
}

// reportAnnotationError logs an annotation problem the first time it is seen. Keying
// on the error text, which names the offending annotation and pattern, collapses the
// same typo templated across many workloads into one line.
func (s *Service) reportAnnotationError(err error, wl workload.Workload, namespace string) {
	if _, seen := s.reportedAnnotationErrors.LoadOrStore(err.Error(), struct{}{}); seen {
		return
	}
	s.log.Error(
		err, "problem evaluating reload annotations",
		"workload", wl.GetName(), "kind", wl.Kind(), "namespace", namespace,
	)
}
