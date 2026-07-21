package metadata

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stakater/Reloader/internal/pkg/workload"
	"github.com/stakater/Reloader/pkg/config"
)

// Publisher handles creating and updating the metadata ConfigMap.
type Publisher struct {
	client client.Client
	cfg    *config.Config
	log    logr.Logger
}

// NewPublisher creates a new Publisher.
func NewPublisher(c client.Client, cfg *config.Config, log logr.Logger) *Publisher {
	return &Publisher{
		client: c,
		cfg:    cfg,
		log:    log,
	}
}

// Publish creates or updates the metadata ConfigMap.
func (p *Publisher) Publish(ctx context.Context) error {
	namespace := os.Getenv(EnvReloaderNamespace)
	if namespace == "" {
		p.log.Info("RELOADER_NAMESPACE is not set, skipping meta info configmap creation")
		return nil
	}

	metaInfo := NewMetaInfo(p.cfg)
	configMap := metaInfo.ToConfigMap()

	existing := &corev1.ConfigMap{}
	err := p.client.Get(
		ctx, client.ObjectKey{
			Name:      ConfigMapName,
			Namespace: namespace,
		}, existing,
	)

	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing meta info configmap: %w", err)
		}
		p.log.Info("Creating meta info configmap")
		if err := p.client.Create(ctx, configMap, client.FieldOwner(workload.FieldManager)); err != nil {
			return fmt.Errorf("failed to create meta info configmap: %w", err)
		}
		p.log.Info("Meta info configmap created successfully")
		return nil
	}

	p.log.Info("Meta info configmap already exists, updating it")
	existing.Data = configMap.Data
	existing.Labels = configMap.Labels
	if err := p.client.Update(ctx, existing, client.FieldOwner(workload.FieldManager)); err != nil {
		return fmt.Errorf("failed to update meta info configmap: %w", err)
	}
	p.log.Info("Meta info configmap updated successfully")
	return nil
}

// PublishMetaInfoConfigMap is a convenience function that creates a Publisher and calls Publish.
func PublishMetaInfoConfigMap(ctx context.Context, c client.Client, cfg *config.Config, log logr.Logger) error {
	publisher := NewPublisher(c, cfg, log)
	return publisher.Publish(ctx)
}

// Runnable returns a controller-runtime Runnable that publishes the meta-info
// ConfigMap when the manager starts. It builds its own uncached client from the
// given rest config and scheme: the ConfigMap lives in Reloader's own namespace,
// which the manager cache does not cover in scoped mode, so a cache-backed client
// cannot read or write it there. Meta-info is internal instance metadata and is
// always published regardless of which resources are watched.
func Runnable(restConfig *rest.Config, scheme *runtime.Scheme, cfg *config.Config, log logr.Logger) RunnableFunc {
	return func(ctx context.Context) error {
		c, err := client.New(restConfig, client.Options{Scheme: scheme})
		if err != nil {
			log.Error(err, "Failed to create client for meta info configmap publisher")
			// Non-fatal, don't return error to avoid crashing the manager
			<-ctx.Done()
			return nil
		}
		if err := PublishMetaInfoConfigMap(ctx, c, cfg, log); err != nil {
			log.Error(err, "Failed to create metadata ConfigMap")
			// Non-fatal, don't return error to avoid crashing the manager
		}
		<-ctx.Done()
		return nil
	}
}

// RunnableFunc is a function that implements the controller-runtime Runnable interface.
type RunnableFunc func(context.Context) error

// Start implements the Runnable interface.
func (r RunnableFunc) Start(ctx context.Context) error {
	return r(ctx)
}
