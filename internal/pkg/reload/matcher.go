package reload

import (
	"regexp"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/config"
)

// ResourceType represents the type of Kubernetes resource.
type ResourceType string

const (
	// ResourceTypeConfigMap represents a ConfigMap resource.
	ResourceTypeConfigMap ResourceType = "configmap"
	// ResourceTypeSecret represents a Secret resource.
	ResourceTypeSecret ResourceType = "secret"
)

// MatchResult contains the result of checking if a workload should be reloaded.
type MatchResult struct {
	// ShouldReload indicates whether the workload should be reloaded.
	ShouldReload bool
	// AutoReload indicates if this is an auto-reload (vs explicit annotation).
	// This affects which container to target for env var injection.
	AutoReload bool
	// Reason provides a human-readable explanation of the decision.
	Reason string
}

// Matcher determines whether a workload should be reloaded based on annotations.
type Matcher struct {
	cfg *config.Config
}

// NewMatcher creates a new Matcher with the given configuration.
func NewMatcher(cfg *config.Config) *Matcher {
	return &Matcher{cfg: cfg}
}

// MatchInput contains all the information needed to determine if a reload should occur.
type MatchInput struct {
	// ResourceName is the name of the ConfigMap or Secret that changed.
	ResourceName string
	// ResourceNamespace is the namespace of the ConfigMap or Secret.
	ResourceNamespace string
	// ResourceType is whether this is a ConfigMap or Secret.
	ResourceType ResourceType
	// ResourceAnnotations are the annotations on the ConfigMap or Secret.
	ResourceAnnotations map[string]string
	// WorkloadAnnotations are the annotations on the workload (Deployment, etc.).
	WorkloadAnnotations map[string]string
	// PodAnnotations are the annotations on the pod template.
	PodAnnotations map[string]string
}

// ShouldReload determines if a workload should be reloaded based on its annotations.
//
// The matching logic follows this precedence (BUG FIX: explicit annotations checked first):
//  1. If the resource has the ignore annotation, skip it
//  2. If the resource is in the exclude list for this workload, skip it
//  3. If explicit reload annotation matches the resource name, reload (not auto)
//  4. If search annotation is enabled and resource has match annotation, reload (auto)
//  5. If auto annotation is "true", reload (auto)
//  6. If typed auto annotation is "true", reload (auto)
//  7. If AutoReloadAll is enabled and no explicit "false" annotations, reload (auto)
func (m *Matcher) ShouldReload(input MatchInput) MatchResult {
	// Check resource-level ignore annotation
	if m.isResourceIgnored(input.ResourceAnnotations) {
		return MatchResult{
			ShouldReload: false,
			Reason:       "resource has ignore annotation",
		}
	}

	// Determine which annotations to use (workload or pod template)
	annotations := m.selectAnnotations(input)

	// Check if resource is excluded
	if m.isResourceExcluded(input.ResourceName, input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: false,
			Reason:       "resource is in exclude list",
		}
	}

	// Check explicit reload annotation (e.g., configmap.reloader.stakater.com/reload: "my-config")
	// BUG FIX: Check this BEFORE auto annotations to ensure explicit references take precedence
	if m.matchesExplicitAnnotation(input.ResourceName, input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   false,
			Reason:       "matches explicit reload annotation",
		}
	}

	// Check search/match pattern
	if m.matchesSearchPattern(input.ResourceAnnotations, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   true,
			Reason:       "matches search/match pattern",
		}
	}

	// Check auto annotations
	if m.matchesAutoAnnotation(input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   true,
			Reason:       "auto annotation enabled",
		}
	}

	// Check global auto-reload-all setting
	if m.matchesAutoReloadAll(input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   true,
			Reason:       "auto-reload-all enabled",
		}
	}

	return MatchResult{
		ShouldReload: false,
		Reason:       "no matching annotations",
	}
}

// isResourceIgnored checks if the resource has the ignore annotation set to true.
func (m *Matcher) isResourceIgnored(resourceAnnotations map[string]string) bool {
	if resourceAnnotations == nil {
		return false
	}
	return resourceAnnotations[m.cfg.Annotations.Ignore] == "true"
}

// selectAnnotations determines which set of annotations to use for matching.
// If workload annotations don't have relevant annotations, fall back to pod annotations.
func (m *Matcher) selectAnnotations(input MatchInput) map[string]string {
	// Check if any relevant annotation exists on workload annotations
	if m.hasRelevantAnnotations(input.WorkloadAnnotations, input.ResourceType) {
		return input.WorkloadAnnotations
	}
	// Fall back to pod annotations
	if m.hasRelevantAnnotations(input.PodAnnotations, input.ResourceType) {
		return input.PodAnnotations
	}
	// Default to workload annotations even if empty
	return input.WorkloadAnnotations
}

// hasRelevantAnnotations checks if the annotations contain any reload-related annotation.
func (m *Matcher) hasRelevantAnnotations(annotations map[string]string, resourceType ResourceType) bool {
	if annotations == nil {
		return false
	}

	// Check for explicit annotation
	explicitAnn := m.getExplicitAnnotation(resourceType)
	if _, ok := annotations[explicitAnn]; ok {
		return true
	}

	// Check for search annotation
	if _, ok := annotations[m.cfg.Annotations.Search]; ok {
		return true
	}

	// Check for auto annotation
	if _, ok := annotations[m.cfg.Annotations.Auto]; ok {
		return true
	}

	// Check for typed auto annotation
	typedAutoAnn := m.getTypedAutoAnnotation(resourceType)
	if _, ok := annotations[typedAutoAnn]; ok {
		return true
	}

	return false
}

// isResourceExcluded checks if the resource is in the exclude list.
func (m *Matcher) isResourceExcluded(resourceName string, resourceType ResourceType, annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	var excludeAnn string
	switch resourceType {
	case ResourceTypeConfigMap:
		excludeAnn = m.cfg.Annotations.ConfigmapExclude
	case ResourceTypeSecret:
		excludeAnn = m.cfg.Annotations.SecretExclude
	}

	excludeList, ok := annotations[excludeAnn]
	if !ok || excludeList == "" {
		return false
	}

	for _, excluded := range strings.Split(excludeList, ",") {
		if strings.TrimSpace(excluded) == resourceName {
			return true
		}
	}

	return false
}

// matchesExplicitAnnotation checks if the resource name matches the explicit reload annotation.
func (m *Matcher) matchesExplicitAnnotation(resourceName string, resourceType ResourceType, annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	explicitAnn := m.getExplicitAnnotation(resourceType)
	annotationValue, ok := annotations[explicitAnn]
	if !ok || annotationValue == "" {
		return false
	}

	// Support comma-separated list of resource names with regex matching
	for _, value := range strings.Split(annotationValue, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		// Support regex patterns
		re, err := regexp.Compile("^" + value + "$")
		if err != nil {
			// If regex is invalid, fall back to exact match
			if value == resourceName {
				return true
			}
			continue
		}
		if re.MatchString(resourceName) {
			return true
		}
	}

	return false
}

// matchesSearchPattern checks if the search/match pattern is enabled.
func (m *Matcher) matchesSearchPattern(resourceAnnotations, workloadAnnotations map[string]string) bool {
	if workloadAnnotations == nil || resourceAnnotations == nil {
		return false
	}

	searchValue, ok := workloadAnnotations[m.cfg.Annotations.Search]
	if !ok || searchValue != "true" {
		return false
	}

	matchValue, ok := resourceAnnotations[m.cfg.Annotations.Match]
	return ok && matchValue == "true"
}

// matchesAutoAnnotation checks if auto reload is enabled via annotations.
func (m *Matcher) matchesAutoAnnotation(resourceType ResourceType, annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	// Check generic auto annotation
	if annotations[m.cfg.Annotations.Auto] == "true" {
		return true
	}

	// Check typed auto annotation
	typedAutoAnn := m.getTypedAutoAnnotation(resourceType)
	return annotations[typedAutoAnn] == "true"
}

// matchesAutoReloadAll checks if global auto-reload-all is enabled.
func (m *Matcher) matchesAutoReloadAll(resourceType ResourceType, annotations map[string]string) bool {
	if !m.cfg.AutoReloadAll {
		return false
	}

	// If auto annotation is explicitly set to false, don't auto-reload
	if annotations != nil {
		if annotations[m.cfg.Annotations.Auto] == "false" {
			return false
		}
		typedAutoAnn := m.getTypedAutoAnnotation(resourceType)
		if annotations[typedAutoAnn] == "false" {
			return false
		}
	}

	return true
}

// getExplicitAnnotation returns the explicit reload annotation for the resource type.
func (m *Matcher) getExplicitAnnotation(resourceType ResourceType) string {
	switch resourceType {
	case ResourceTypeConfigMap:
		return m.cfg.Annotations.ConfigmapReload
	case ResourceTypeSecret:
		return m.cfg.Annotations.SecretReload
	default:
		return ""
	}
}

// getTypedAutoAnnotation returns the typed auto annotation for the resource type.
func (m *Matcher) getTypedAutoAnnotation(resourceType ResourceType) string {
	switch resourceType {
	case ResourceTypeConfigMap:
		return m.cfg.Annotations.ConfigmapAuto
	case ResourceTypeSecret:
		return m.cfg.Annotations.SecretAuto
	default:
		return ""
	}
}
