package util

import "time"

type ReloadSource struct {
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Hash          string   `json:"hash"`
	ContainerRefs []string `json:"containerRefs"`
	ObservedAt    int64    `json:"observedAt"`
}

func NewReloadSource(
	resourceName string,
	resourceNamespace string,
	resourceType string,
	resourceHash string,
	containerRefs []string,
) ReloadSource {
	return ReloadSource{
		ObservedAt:    time.Now().Unix(),
		Name:          resourceName,
		Namespace:     resourceNamespace,
		Type:          resourceType,
		Hash:          resourceHash,
		ContainerRefs: containerRefs,
	}
}

func NewReloadSourceFromConfig(config Config, containerRefs []string) ReloadSource {
	return NewReloadSource(
		config.ResourceName,
		config.Namespace,
		config.Type,
		config.SHAValue,
		containerRefs,
	)
}
