package config

import (
	"fmt"
	"strings"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"k8s.io/apimachinery/pkg/labels"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config.%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var b strings.Builder
	b.WriteString("multiple configuration errors:\n")
	for _, err := range e {
		b.WriteString("  - ")
		b.WriteString(err.Error())
		b.WriteString("\n")
	}
	return b.String()
}

// Validate checks the configuration for errors and normalizes values.
func (c *Config) Validate() error {
	var errs ValidationErrors

	// Validate ReloadStrategy
	switch c.ReloadStrategy {
	case ReloadStrategyEnvVars, ReloadStrategyAnnotations:
		// valid
	case "":
		c.ReloadStrategy = ReloadStrategyEnvVars
	default:
		errs = append(
			errs, ValidationError{
				Field:   "ReloadStrategy",
				Message: fmt.Sprintf("invalid value %q, must be %q or %q", c.ReloadStrategy, ReloadStrategyEnvVars, ReloadStrategyAnnotations),
			},
		)
	}

	// Validate ArgoRolloutStrategy
	switch c.ArgoRolloutStrategy {
	case ArgoRolloutStrategyRestart, ArgoRolloutStrategyRollout:
		// valid
	case "":
		c.ArgoRolloutStrategy = ArgoRolloutStrategyRollout
	default:
		errs = append(
			errs, ValidationError{
				Field: "ArgoRolloutStrategy",
				Message: fmt.Sprintf(
					"invalid value %q, must be %q or %q", c.ArgoRolloutStrategy, ArgoRolloutStrategyRestart, ArgoRolloutStrategyRollout,
				),
			},
		)
	}

	// Validate LogLevel
	switch strings.ToLower(c.LogLevel) {
	case "trace", "debug", "info", "warn", "warning", "error", "fatal", "panic", "":
		// valid
	default:
		errs = append(
			errs, ValidationError{
				Field:   "LogLevel",
				Message: fmt.Sprintf("invalid log level %q", c.LogLevel),
			},
		)
	}

	// Validate LogFormat
	switch strings.ToLower(c.LogFormat) {
	case "json", "":
		// valid
	default:
		errs = append(
			errs, ValidationError{
				Field:   "LogFormat",
				Message: fmt.Sprintf("invalid log format %q, must be \"json\" or empty", c.LogFormat),
			},
		)
	}

	// Normalize IgnoredResources to lowercase for consistent comparison
	c.IgnoredResources = normalizeToLower(c.IgnoredResources)

	// Validate and normalize IgnoredWorkloads
	c.IgnoredWorkloads = normalizeToLower(c.IgnoredWorkloads)
	for _, w := range c.IgnoredWorkloads {
		if _, err := workload.KindFromString(w); err != nil {
			errs = append(errs, ValidationError{
				Field:   "IgnoredWorkloads",
				Message: fmt.Sprintf("unknown workload type %q", w),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// normalizeToLower converts all strings in the slice to lowercase and removes empty strings.
func normalizeToLower(items []string) []string {
	if len(items) == 0 {
		return items
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(strings.ToLower(item))
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// ParseSelectors parses a slice of selector strings into label selectors.
func ParseSelectors(selectorStrings []string) ([]labels.Selector, error) {
	if len(selectorStrings) == 0 {
		return nil, nil
	}

	selectors := make([]labels.Selector, 0, len(selectorStrings))
	for _, s := range selectorStrings {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		selector, err := labels.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid selector %q: %w", s, err)
		}
		selectors = append(selectors, selector)
	}
	return selectors, nil
}
