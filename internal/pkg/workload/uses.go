package workload

import corev1 "k8s.io/api/core/v1"

// SpecUsesConfigMap checks if a PodSpec references the named ConfigMap.
func SpecUsesConfigMap(spec *corev1.PodSpec, name string) bool {
	for _, vol := range spec.Volumes {
		if vol.ConfigMap != nil && vol.ConfigMap.Name == name {
			return true
		}
		if vol.Projected != nil {
			for _, source := range vol.Projected.Sources {
				if source.ConfigMap != nil && source.ConfigMap.Name == name {
					return true
				}
			}
		}
	}

	if containersUseConfigMap(spec.Containers, name) {
		return true
	}
	return containersUseConfigMap(spec.InitContainers, name)
}

func containersUseConfigMap(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}
	return false
}

// SpecUsesSecret checks if a PodSpec references the named Secret.
func SpecUsesSecret(spec *corev1.PodSpec, name string) bool {
	for _, vol := range spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == name {
			return true
		}
		if vol.Projected != nil {
			for _, source := range vol.Projected.Sources {
				if source.Secret != nil && source.Secret.Name == name {
					return true
				}
			}
		}
	}

	if containersUseSecret(spec.Containers, name) {
		return true
	}
	return containersUseSecret(spec.InitContainers, name)
}

func containersUseSecret(containers []corev1.Container, name string) bool {
	for _, container := range containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil && envFrom.SecretRef.Name == name {
				return true
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		}
	}
	return false
}
