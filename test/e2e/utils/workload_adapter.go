package utils

import (
	"context"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// WorkloadType represents the type of Kubernetes workload.
type WorkloadType string

const (
	WorkloadDeployment       WorkloadType = "Deployment"
	WorkloadDaemonSet        WorkloadType = "DaemonSet"
	WorkloadStatefulSet      WorkloadType = "StatefulSet"
	WorkloadCronJob          WorkloadType = "CronJob"
	WorkloadJob              WorkloadType = "Job"
	WorkloadArgoRollout      WorkloadType = "ArgoRollout"
	WorkloadDeploymentConfig WorkloadType = "DeploymentConfig"
)

// ReloadStrategy represents the reload strategy used by Reloader.
type ReloadStrategy string

const (
	StrategyAnnotations ReloadStrategy = "annotations"
	StrategyEnvVars     ReloadStrategy = "envvars"
)

// WorkloadConfig holds configuration for workload creation.
type WorkloadConfig struct {
	// Resource references
	ConfigMapName string
	SecretName    string

	// Annotations to set on the workload
	Annotations map[string]string

	// Reference methods (flags - multiple can be true)
	UseConfigMapEnvFrom    bool
	UseSecretEnvFrom       bool
	UseConfigMapVolume     bool
	UseSecretVolume        bool
	UseProjectedVolume     bool
	UseConfigMapKeyRef     bool
	UseSecretKeyRef        bool
	UseInitContainer       bool
	UseInitContainerVolume bool

	// For valueFrom references
	ConfigMapKey string
	SecretKey    string
	EnvVarName   string

	// Special options
	MultipleContainers int // Number of containers (0 or 1 means single container)
}

// WorkloadAdapter provides a unified interface for all workload types.
// This allows tests to be parameterized across different workload types.
type WorkloadAdapter interface {
	// Type returns the workload type.
	Type() WorkloadType

	// Create creates the workload with the given config.
	Create(ctx context.Context, namespace, name string, cfg WorkloadConfig) error

	// Delete removes the workload.
	Delete(ctx context.Context, namespace, name string) error

	// WaitReady waits for the workload to be ready.
	WaitReady(ctx context.Context, namespace, name string, timeout time.Duration) error

	// WaitReloaded waits for the workload to have the reload annotation.
	// Returns true if the annotation was found, false if timeout occurred.
	WaitReloaded(ctx context.Context, namespace, name, annotationKey string, timeout time.Duration) (bool, error)

	// WaitEnvVar waits for the workload to have a STAKATER_ env var (for envvars strategy).
	// Returns true if the env var was found, false if timeout occurred.
	WaitEnvVar(ctx context.Context, namespace, name, prefix string, timeout time.Duration) (bool, error)

	// SupportsEnvVarStrategy returns true if the workload supports env var reload strategy.
	// CronJob does not support this as it uses job creation instead.
	SupportsEnvVarStrategy() bool

	// RequiresSpecialHandling returns true for workloads that need special handling.
	// For example, CronJob triggers a new job instead of rolling restart.
	RequiresSpecialHandling() bool
}

// AdapterRegistry holds adapters for all workload types.
type AdapterRegistry struct {
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface
	adapters      map[WorkloadType]WorkloadAdapter
}

// NewAdapterRegistry creates a new adapter registry with all standard adapters.
func NewAdapterRegistry(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface) *AdapterRegistry {
	r := &AdapterRegistry{
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		adapters:      make(map[WorkloadType]WorkloadAdapter),
	}

	// Register standard adapters
	r.adapters[WorkloadDeployment] = NewDeploymentAdapter(kubeClient)
	r.adapters[WorkloadDaemonSet] = NewDaemonSetAdapter(kubeClient)
	r.adapters[WorkloadStatefulSet] = NewStatefulSetAdapter(kubeClient)
	r.adapters[WorkloadCronJob] = NewCronJobAdapter(kubeClient)
	r.adapters[WorkloadJob] = NewJobAdapter(kubeClient)

	// Argo and OpenShift adapters are registered separately via RegisterAdapter
	// as they require specific cluster support

	return r
}

// RegisterAdapter registers a custom adapter for a workload type.
// Use this to add Argo Rollout or DeploymentConfig adapters.
func (r *AdapterRegistry) RegisterAdapter(adapter WorkloadAdapter) {
	r.adapters[adapter.Type()] = adapter
}

// Get returns the adapter for the given workload type.
// Returns nil if the adapter is not registered.
func (r *AdapterRegistry) Get(wt WorkloadType) WorkloadAdapter {
	return r.adapters[wt]
}

// GetStandardWorkloads returns the standard workload types that are always available.
func (r *AdapterRegistry) GetStandardWorkloads() []WorkloadType {
	return []WorkloadType{
		WorkloadDeployment,
		WorkloadDaemonSet,
		WorkloadStatefulSet,
	}
}

// GetAllWorkloads returns all registered workload types.
func (r *AdapterRegistry) GetAllWorkloads() []WorkloadType {
	result := make([]WorkloadType, 0, len(r.adapters))
	for wt := range r.adapters {
		result = append(result, wt)
	}
	return result
}

// GetEnvVarWorkloads returns workload types that support env var reload strategy.
func (r *AdapterRegistry) GetEnvVarWorkloads() []WorkloadType {
	result := make([]WorkloadType, 0)
	for wt, adapter := range r.adapters {
		if adapter.SupportsEnvVarStrategy() {
			result = append(result, wt)
		}
	}
	return result
}
