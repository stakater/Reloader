package utils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// AddEnvFromSource adds ConfigMap or Secret envFrom to a container.
func AddEnvFromSource(spec *corev1.PodSpec, containerIdx int, name string, isSecret bool) {
	if containerIdx >= len(spec.Containers) {
		return
	}
	source := corev1.EnvFromSource{}
	if isSecret {
		source.SecretRef = &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
		}
	} else {
		source.ConfigMapRef = &corev1.ConfigMapEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
		}
	}
	spec.Containers[containerIdx].EnvFrom = append(spec.Containers[containerIdx].EnvFrom, source)
}

// AddVolume adds a volume and mount to a container.
func AddVolume(spec *corev1.PodSpec, containerIdx int, volume corev1.Volume, mountPath string) {
	spec.Volumes = append(spec.Volumes, volume)
	if containerIdx < len(spec.Containers) {
		spec.Containers[containerIdx].VolumeMounts = append(
			spec.Containers[containerIdx].VolumeMounts,
			corev1.VolumeMount{Name: volume.Name, MountPath: mountPath},
		)
	}
}

// AddConfigMapVolume adds ConfigMap volume and mount.
func AddConfigMapVolume(spec *corev1.PodSpec, containerIdx int, name string) {
	AddVolume(spec, containerIdx, corev1.Volume{
		Name: "cm-" + name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
			},
		},
	}, "/etc/config/"+name)
}

// AddSecretVolume adds Secret volume and mount.
func AddSecretVolume(spec *corev1.PodSpec, containerIdx int, name string) {
	AddVolume(spec, containerIdx, corev1.Volume{
		Name: "secret-" + name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: name},
		},
	}, "/etc/secrets/"+name)
}

// AddProjectedVolume adds projected volume with ConfigMap and/or Secret.
func AddProjectedVolume(spec *corev1.PodSpec, containerIdx int, cmName, secretName string) {
	sources := []corev1.VolumeProjection{}
	if cmName != "" {
		sources = append(sources, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
			},
		})
	}
	if secretName != "" {
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		})
	}
	AddVolume(spec, containerIdx, corev1.Volume{
		Name: "projected-config",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{Sources: sources},
		},
	}, "/etc/projected")
}

// AddKeyRef adds env var from ConfigMap or Secret key.
func AddKeyRef(spec *corev1.PodSpec, containerIdx int, resourceName, key, envVarName string, isSecret bool) {
	if containerIdx >= len(spec.Containers) {
		return
	}
	envVar := corev1.EnvVar{Name: envVarName}
	if isSecret {
		envVar.ValueFrom = &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: resourceName},
				Key:                  key,
			},
		}
	} else {
		envVar.ValueFrom = &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: resourceName},
				Key:                  key,
			},
		}
	}
	spec.Containers[containerIdx].Env = append(spec.Containers[containerIdx].Env, envVar)
}

// AddCSIVolume adds CSI volume referencing SecretProviderClass.
func AddCSIVolume(spec *corev1.PodSpec, containerIdx int, spcName string) {
	volumeName := "csi-" + spcName
	mountPath := "/mnt/secrets-store/" + spcName
	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   CSIDriverName,
				ReadOnly: ptr.To(true),
				VolumeAttributes: map[string]string{
					"secretProviderClass": spcName,
				},
			},
		},
	})
	if containerIdx < len(spec.Containers) {
		spec.Containers[containerIdx].VolumeMounts = append(
			spec.Containers[containerIdx].VolumeMounts,
			corev1.VolumeMount{Name: volumeName, MountPath: mountPath, ReadOnly: true},
		)
	}
}

// AddInitContainer adds init container with optional envFrom references.
func AddInitContainer(spec *corev1.PodSpec, cmName, secretName string) {
	init := corev1.Container{
		Name:    "init",
		Image:   DefaultImage,
		Command: []string{"sh", "-c", "echo init done"},
	}
	if cmName != "" {
		init.EnvFrom = append(init.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
			},
		})
	}
	if secretName != "" {
		init.EnvFrom = append(init.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		})
	}
	spec.InitContainers = append(spec.InitContainers, init)
}

// AddInitContainerWithVolumes adds init container with volume mounts.
func AddInitContainerWithVolumes(spec *corev1.PodSpec, cmName, secretName string) {
	init := corev1.Container{
		Name:    "init",
		Image:   DefaultImage,
		Command: []string{"sh", "-c", "echo init done"},
	}
	if cmName != "" {
		volumeName := "init-cm-" + cmName
		spec.Volumes = append(spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
		init.VolumeMounts = append(init.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/etc/init-config/" + cmName,
		})
	}
	if secretName != "" {
		volumeName := "init-secret-" + secretName
		spec.Volumes = append(spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: secretName},
			},
		})
		init.VolumeMounts = append(init.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/etc/init-secrets/" + secretName,
		})
	}
	spec.InitContainers = append(spec.InitContainers, init)
}

// ApplyWorkloadConfig applies all WorkloadConfig settings to a PodTemplateSpec.
// This includes both pod template annotations and pod spec configuration.
func ApplyWorkloadConfig(template *corev1.PodTemplateSpec, cfg WorkloadConfig) {
	// Apply pod template annotations
	if len(cfg.PodTemplateAnnotations) > 0 {
		if template.Annotations == nil {
			template.Annotations = make(map[string]string)
		}
		for k, v := range cfg.PodTemplateAnnotations {
			template.Annotations[k] = v
		}
	}

	// Apply pod spec configuration
	spec := &template.Spec
	if cfg.UseConfigMapEnvFrom && cfg.ConfigMapName != "" {
		AddEnvFromSource(spec, 0, cfg.ConfigMapName, false)
	}
	if cfg.UseSecretEnvFrom && cfg.SecretName != "" {
		AddEnvFromSource(spec, 0, cfg.SecretName, true)
	}
	if cfg.UseConfigMapVolume && cfg.ConfigMapName != "" {
		AddConfigMapVolume(spec, 0, cfg.ConfigMapName)
	}
	if cfg.UseSecretVolume && cfg.SecretName != "" {
		AddSecretVolume(spec, 0, cfg.SecretName)
	}
	if cfg.UseProjectedVolume {
		AddProjectedVolume(spec, 0, cfg.ConfigMapName, cfg.SecretName)
	}
	if cfg.UseConfigMapKeyRef && cfg.ConfigMapName != "" {
		key := cfg.ConfigMapKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "CONFIG_VAR"
		}
		AddKeyRef(spec, 0, cfg.ConfigMapName, key, envVar, false)
	}
	if cfg.UseSecretKeyRef && cfg.SecretName != "" {
		key := cfg.SecretKey
		if key == "" {
			key = "key"
		}
		envVar := cfg.EnvVarName
		if envVar == "" {
			envVar = "SECRET_VAR"
		}
		AddKeyRef(spec, 0, cfg.SecretName, key, envVar, true)
	}
	if cfg.UseCSIVolume && cfg.SPCName != "" {
		AddCSIVolume(spec, 0, cfg.SPCName)
	}
	if cfg.UseInitContainer {
		AddInitContainer(spec, cfg.ConfigMapName, cfg.SecretName)
	}
	if cfg.UseInitContainerVolume {
		AddInitContainerWithVolumes(spec, cfg.ConfigMapName, cfg.SecretName)
	}
	if cfg.UseInitContainerCSI && cfg.SPCName != "" {
		AddCSIVolume(spec, 0, cfg.SPCName)
	}
	if cfg.MultipleContainers > 1 {
		for i := 1; i < cfg.MultipleContainers; i++ {
			spec.Containers = append(spec.Containers, corev1.Container{
				Name:    fmt.Sprintf("container-%d", i),
				Image:   DefaultImage,
				Command: []string{"sh", "-c", DefaultCommand},
			})
		}
	}
}
