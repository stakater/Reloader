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
	ShouldReload bool
	AutoReload   bool
	Reason       string
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
	ResourceName        string
	ResourceNamespace   string
	ResourceType        ResourceType
	ResourceAnnotations map[string]string
	WorkloadAnnotations map[string]string
	PodAnnotations      map[string]string
}

// ShouldReload determines if a workload should be reloaded based on its annotations.
func (m *Matcher) ShouldReload(input MatchInput) MatchResult {
	if m.isResourceIgnored(input.ResourceAnnotations) {
		return MatchResult{
			ShouldReload: false,
			Reason:       "resource has ignore annotation",
		}
	}

	annotations := m.selectAnnotations(input)

	if m.isResourceExcluded(input.ResourceName, input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: false,
			Reason:       "resource is in exclude list",
		}
	}

	if m.matchesExplicitAnnotation(input.ResourceName, input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   false,
			Reason:       "matches explicit reload annotation",
		}
	}

	if m.matchesSearchPattern(input.ResourceAnnotations, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   true,
			Reason:       "matches search/match pattern",
		}
	}

	if m.matchesAutoAnnotation(input.ResourceType, annotations) {
		return MatchResult{
			ShouldReload: true,
			AutoReload:   true,
			Reason:       "auto annotation enabled",
		}
	}

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

func (m *Matcher) isResourceIgnored(resourceAnnotations map[string]string) bool {
	if resourceAnnotations == nil {
		return false
	}
	return resourceAnnotations[m.cfg.Annotations.Ignore] == "true"
}

func (m *Matcher) selectAnnotations(input MatchInput) map[string]string {
	if m.hasRelevantAnnotations(input.WorkloadAnnotations, input.ResourceType) {
		return input.WorkloadAnnotations
	}
	if m.hasRelevantAnnotations(input.PodAnnotations, input.ResourceType) {
		return input.PodAnnotations
	}
	return input.WorkloadAnnotations
}

func (m *Matcher) hasRelevantAnnotations(annotations map[string]string, resourceType ResourceType) bool {
	if annotations == nil {
		return false
	}

	explicitAnn := m.getExplicitAnnotation(resourceType)
	if _, ok := annotations[explicitAnn]; ok {
		return true
	}

	if _, ok := annotations[m.cfg.Annotations.Search]; ok {
		return true
	}

	if _, ok := annotations[m.cfg.Annotations.Auto]; ok {
		return true
	}

	typedAutoAnn := m.getTypedAutoAnnotation(resourceType)
	if _, ok := annotations[typedAutoAnn]; ok {
		return true
	}

	return false
}

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

func (m *Matcher) matchesExplicitAnnotation(resourceName string, resourceType ResourceType, annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	explicitAnn := m.getExplicitAnnotation(resourceType)
	annotationValue, ok := annotations[explicitAnn]
	if !ok || annotationValue == "" {
		return false
	}

	for _, value := range strings.Split(annotationValue, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		re, err := regexp.Compile("^" + value + "$")
		if err != nil {
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

func (m *Matcher) matchesAutoAnnotation(resourceType ResourceType, annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	if annotations[m.cfg.Annotations.Auto] == "true" {
		return true
	}

	typedAutoAnn := m.getTypedAutoAnnotation(resourceType)
	return annotations[typedAutoAnn] == "true"
}

func (m *Matcher) matchesAutoReloadAll(resourceType ResourceType, annotations map[string]string) bool {
	if !m.cfg.AutoReloadAll {
		return false
	}

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
