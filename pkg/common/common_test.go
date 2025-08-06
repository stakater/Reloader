package common

import (
	"testing"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/internal/pkg/testutil"
	"github.com/stakater/Reloader/internal/pkg/util"
)

func TestShouldReload(t *testing.T) {
	tests := []struct {
		name           string
		config         util.Config
		resourceType   string
		annotations    Map
		podAnnotations Map
		options        *ReloaderOptions
		expectedReload bool
		expectedAuto   bool
	}{
		{
			name: "should ignore namespace when in NamespacesToIgnore",
			config: util.Config{
				Namespace:    "ignored-namespace",
				ResourceName: "test-config",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			resourceType:   "configmaps",
			annotations:    Map{},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				NamespacesToIgnore: []string{"ignored-namespace", "other-namespace"},
			},
			expectedReload: false,
			expectedAuto:   false,
		},
		{
			name: "should ignore resource when in ResourcesToIgnore",
			config: util.Config{
				Namespace:    "default",
				ResourceName: "test-config",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			resourceType:   testutil.ConfigmapNamePrefix,
			annotations:    Map{},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ResourcesToIgnore: []string{testutil.ConfigmapResourceType},
			},
			expectedReload: false,
			expectedAuto:   false,
		},
		{
			name: "should ignore Rollout when IsArgoRollouts is false",
			config: util.Config{
				Namespace:    "default",
				ResourceName: "test-rollout",
				Type:         constants.ConfigmapEnvVarPostfix,
			},
			resourceType:   "Rollout",
			annotations:    Map{},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				IsArgoRollouts: false,
			},
			expectedReload: false,
			expectedAuto:   false,
		},
		{
			name: "should allow Rollout when IsArgoRollouts is true",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-rollout",
				Type:                constants.ConfigmapEnvVarPostfix,
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "Rollout",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				IsArgoRollouts:                  true,
				ReloaderAutoAnnotation:          "reloader.stakater.com/auto",
				ConfigmapReloaderAutoAnnotation: "configmap.reloader.stakater.com/auto",
				IgnoreResourceAnnotation:        "reloader.stakater.com/ignore",
			},
			expectedReload: true,
			expectedAuto:   true,
		},
		{
			name: "should filter by ResourceSelectors - matching labels",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-config",
				Type:                constants.ConfigmapEnvVarPostfix,
				Labels:              map[string]string{"app": "test", "env": "prod"},
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "configmaps",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ResourceSelectors:               []string{"app=test,env=prod"},
				ReloaderAutoAnnotation:          "reloader.stakater.com/auto",
				ConfigmapReloaderAutoAnnotation: "configmap.reloader.stakater.com/auto",
				IgnoreResourceAnnotation:        "reloader.stakater.com/ignore",
			},
			expectedReload: true,
			expectedAuto:   true,
		},
		{
			name: "should filter by ResourceSelectors - non-matching labels",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-config",
				Type:                constants.ConfigmapEnvVarPostfix,
				Labels:              map[string]string{"app": "test", "env": "dev"},
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "configmaps",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ResourceSelectors:      []string{"app=test,env=prod"},
				ReloaderAutoAnnotation: "reloader.stakater.com/auto",
			},
			expectedReload: false,
			expectedAuto:   false,
		},
		{
			name: "should proceed to ShouldReloadInternal when all filters pass",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-config",
				Type:                constants.ConfigmapEnvVarPostfix,
				Labels:              map[string]string{"app": "test"},
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "configmaps",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ReloaderAutoAnnotation:          "reloader.stakater.com/auto",
				ConfigmapReloaderAutoAnnotation: "configmap.reloader.stakater.com/auto",
				IgnoreResourceAnnotation:        "reloader.stakater.com/ignore",
				IsArgoRollouts:                  true,
			},
			expectedReload: true,
			expectedAuto:   true,
		},
		{
			name: "should not reload when multiple filters block the resource",
			config: util.Config{
				Namespace:    "ignored-namespace",
				ResourceName: "test-config",
				Type:         constants.ConfigmapEnvVarPostfix,
				Labels:       map[string]string{"app": "wrong"},
			},
			resourceType: "configmaps",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				NamespacesToIgnore:     []string{"ignored-namespace"},
				ResourceSelectors:      []string{"app=test"},
				ResourcesToIgnore:      []string{"configmaps"},
				ReloaderAutoAnnotation: "reloader.stakater.com/auto",
			},
			expectedReload: false,
			expectedAuto:   false,
		},
		{
			name: "should handle empty NamespacesToIgnore",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-config",
				Type:                constants.ConfigmapEnvVarPostfix,
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "configmaps",
			annotations: Map{
				"reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				NamespacesToIgnore:              []string{},
				ReloaderAutoAnnotation:          "reloader.stakater.com/auto",
				ConfigmapReloaderAutoAnnotation: "configmap.reloader.stakater.com/auto",
				IgnoreResourceAnnotation:        "reloader.stakater.com/ignore",
			},
			expectedReload: true,
			expectedAuto:   true,
		},
		{
			name: "should handle empty ResourcesToIgnore",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-secret",
				Type:                constants.SecretEnvVarPostfix,
				Annotation:          "secret.reloader.stakater.com/reload",
				TypedAutoAnnotation: "secret.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "secrets",
			annotations: Map{
				"secret.reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ResourcesToIgnore:            []string{},
				SecretReloaderAutoAnnotation: "secret.reloader.stakater.com/auto",
				ReloaderAutoAnnotation:       "reloader.stakater.com/auto",
				IgnoreResourceAnnotation:     "reloader.stakater.com/ignore",
			},
			expectedReload: true,
			expectedAuto:   true,
		},
		{
			name: "should handle empty ResourceSelectors",
			config: util.Config{
				Namespace:           "default",
				ResourceName:        "test-config",
				Type:                constants.ConfigmapEnvVarPostfix,
				Labels:              map[string]string{},
				Annotation:          "configmap.reloader.stakater.com/reload",
				TypedAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ResourceAnnotations: Map{},
			},
			resourceType: "configmaps",
			annotations: Map{
				"configmap.reloader.stakater.com/auto": "true",
			},
			podAnnotations: Map{},
			options: &ReloaderOptions{
				ResourceSelectors:               []string{},
				ConfigmapReloaderAutoAnnotation: "configmap.reloader.stakater.com/auto",
				ReloaderAutoAnnotation:          "reloader.stakater.com/auto",
				IgnoreResourceAnnotation:        "reloader.stakater.com/ignore",
			},
			expectedReload: true,
			expectedAuto:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldReload(tt.config, tt.resourceType, tt.annotations, tt.podAnnotations, tt.options)

			if result.ShouldReload != tt.expectedReload {
				t.Errorf("ShouldReload() ShouldReload = %v, expected %v", result.ShouldReload, tt.expectedReload)
			}

			if result.AutoReload != tt.expectedAuto {
				t.Errorf("ShouldReload() AutoReload = %v, expected %v", result.AutoReload, tt.expectedAuto)
			}
		})
	}
}
