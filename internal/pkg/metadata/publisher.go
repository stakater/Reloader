package metadata

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/stakater/Reloader/internal/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	err := p.client.Get(ctx, client.ObjectKey{
		Name:      ConfigMapName,
		Namespace: namespace,
	}, existing)

	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing meta info configmap: %w", err)
		}
		p.log.Info("Creating meta info configmap")
		if err := p.client.Create(ctx, configMap, client.FieldOwner(FieldManager)); err != nil {
			return fmt.Errorf("failed to create meta info configmap: %w", err)
		}
		p.log.Info("Meta info configmap created successfully")
		return nil
	}

	p.log.Info("Meta info configmap already exists, updating it")
	existing.Data = configMap.Data
	existing.Labels = configMap.Labels
	if err := p.client.Update(ctx, existing, client.FieldOwner(FieldManager)); err != nil {
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

// CreateOrUpdate creates or updates the metadata ConfigMap using the provided client.
func CreateOrUpdate(c client.Client, cfg *config.Config, log logr.Logger) error {
	ctx := context.Background()
	return PublishMetaInfoConfigMap(ctx, c, cfg, log)
}
