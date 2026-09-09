// Copyright 2026 Doska. Licensed under the Apache License, Version 2.0.
package restartwindow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"github.com/stakater/Reloader/internal/pkg/constants"
	"github.com/stakater/Reloader/pkg/common"
)

type Source struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	ObservedHash string `json:"observedHash"`
}
type Target struct{ Kind, Namespace, Name string }
type State map[string]Source

func Supported(kind string) bool {
	return kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet"
}
func HasPolicy(a map[string]string) bool {
	_, ok := a[PolicyAnnotation]
	return ok || (a[PendingAnnotation] != "" && a[PendingAnnotation] != "{}")
}
func key(s Source) string { return s.Type + "/" + s.Name }

func get(ctx context.Context, c kubernetes.Interface, t Target) (runtime.Object, error) {
	switch t.Kind {
	case "Deployment":
		return c.AppsV1().Deployments(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	case "StatefulSet":
		return c.AppsV1().StatefulSets(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	case "DaemonSet":
		return c.AppsV1().DaemonSets(t.Namespace).Get(ctx, t.Name, metav1.GetOptions{})
	}
	return nil, fmt.Errorf("restart windows do not support %s", t.Kind)
}
func patch(ctx context.Context, c kubernetes.Interface, t Target, obj runtime.Object, annotations map[string]any, token string) error {
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	body := map[string]any{"metadata": map[string]any{"uid": m.GetUID(), "resourceVersion": m.GetResourceVersion(), "annotations": annotations}}
	if token != "" {
		body["spec"] = map[string]any{"template": map[string]any{"metadata": map[string]any{"annotations": map[string]string{RestartAnnotation: token}}}}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	switch t.Kind {
	case "Deployment":
		_, err = c.AppsV1().Deployments(t.Namespace).Patch(ctx, t.Name, types.MergePatchType, b, metav1.PatchOptions{})
	case "StatefulSet":
		_, err = c.AppsV1().StatefulSets(t.Namespace).Patch(ctx, t.Name, types.MergePatchType, b, metav1.PatchOptions{})
	case "DaemonSet":
		_, err = c.AppsV1().DaemonSets(t.Namespace).Patch(ctx, t.Name, types.MergePatchType, b, metav1.PatchOptions{})
	default:
		return fmt.Errorf("unsupported workload %s", t.Kind)
	}
	return err
}
func state(a map[string]string) (State, map[string]string, error) {
	pending := State{}
	applied := map[string]string{}
	if raw := a[PendingAnnotation]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &pending); err != nil {
			return nil, nil, err
		}
	}
	if raw := a[AppliedAnnotation]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &applied); err != nil {
			return nil, nil, err
		}
	}
	if pending == nil {
		pending = State{}
	}
	if applied == nil {
		applied = map[string]string{}
	}
	return pending, applied, nil
}
func encode(v any) string { b, _ := json.Marshal(v); return string(b) }

// Request persists references only, never Secret contents. Optimistic concurrency
// prevents losing another source's pending change. Execution uses fresh data.
func Request(ctx context.Context, c kubernetes.Interface, t Target, uid types.UID, s Source) error {
	if !Supported(t.Kind) {
		return fmt.Errorf("restart windows do not support %s", t.Kind)
	}
	if s.Type != constants.SecretEnvVarPostfix && s.Type != constants.ConfigmapEnvVarPostfix {
		return fmt.Errorf("restart windows support Secrets and ConfigMaps only")
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, err := get(ctx, c, t)
		if err != nil {
			return err
		}
		m, _ := meta.Accessor(obj)
		if m.GetUID() != uid {
			return nil
		} // never transfer work to a recreated workload
		p, _, err := state(m.GetAnnotations())
		if err != nil {
			return err
		}
		if old, ok := p[key(s)]; ok && old == s {
			return nil
		}
		if len(p) >= 128 {
			return fmt.Errorf("too many pending sources")
		}
		p[key(s)] = s
		return patch(ctx, c, t, obj, map[string]any{PendingAnnotation: encode(p)}, "")
	})
}
func sourceConfig(ctx context.Context, c kubernetes.Interface, ns string, s Source) (common.Config, error) {
	switch s.Type {
	case constants.SecretEnvVarPostfix:
		x, e := c.CoreV1().Secrets(ns).Get(ctx, s.Name, metav1.GetOptions{})
		if e != nil {
			return common.Config{}, e
		}
		return common.GetSecretConfig(x), nil
	case constants.ConfigmapEnvVarPostfix:
		x, e := c.CoreV1().ConfigMaps(ns).Get(ctx, s.Name, metav1.GetOptions{})
		if e != nil {
			return common.Config{}, e
		}
		return common.GetConfigmapConfig(x), nil
	}
	return common.Config{}, fmt.Errorf("unsupported source type %s", s.Type)
}
func template(obj runtime.Object) *corev1.PodTemplateSpec {
	switch x := obj.(type) {
	case *appsv1.Deployment:
		return &x.Spec.Template
	case *appsv1.StatefulSet:
		return &x.Spec.Template
	case *appsv1.DaemonSet:
		return &x.Spec.Template
	}
	return nil
}
func references(p *corev1.PodSpec, s Source) bool {
	for _, v := range p.Volumes {
		if s.Type == constants.SecretEnvVarPostfix && v.Secret != nil && v.Secret.SecretName == s.Name {
			return true
		}
		if s.Type == constants.ConfigmapEnvVarPostfix && v.ConfigMap != nil && v.ConfigMap.Name == s.Name {
			return true
		}
		if v.Projected != nil {
			for _, x := range v.Projected.Sources {
				if s.Type == constants.SecretEnvVarPostfix && x.Secret != nil && x.Secret.Name == s.Name {
					return true
				}
				if s.Type == constants.ConfigmapEnvVarPostfix && x.ConfigMap != nil && x.ConfigMap.Name == s.Name {
					return true
				}
			}
		}
	}
	containers := append(append([]corev1.Container{}, p.Containers...), p.InitContainers...)
	for _, c := range containers {
		for _, e := range c.EnvFrom {
			if s.Type == constants.SecretEnvVarPostfix && e.SecretRef != nil && e.SecretRef.Name == s.Name {
				return true
			}
			if s.Type == constants.ConfigmapEnvVarPostfix && e.ConfigMapRef != nil && e.ConfigMapRef.Name == s.Name {
				return true
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil {
				if s.Type == constants.SecretEnvVarPostfix && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == s.Name {
					return true
				}
				if s.Type == constants.ConfigmapEnvVarPostfix && e.ValueFrom.ConfigMapKeyRef != nil && e.ValueFrom.ConfigMapKeyRef.Name == s.Name {
					return true
				}
			}
		}
	}
	return false
}

// Reconcile consumes all pending changes in ONE atomic template + metadata patch.
// A lost response/restarted leader cannot repeat an already committed restart.
func Reconcile(ctx context.Context, c kubernetes.Interface, t Target, now func() time.Time, selector labels.Selector, allowedSource func(string) bool, recorder record.EventRecorder) (time.Duration, error) {
	obj, err := get(ctx, c, t)
	if apierrors.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return time.Minute, err
	}
	m, _ := meta.Accessor(obj)
	a := m.GetAnnotations()
	pending, applied, err := state(a)
	if err != nil {
		return time.Minute, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	if raw, present := a[PolicyAnnotation]; present && raw == "" {
		return time.Minute, fmt.Errorf("restart-window annotation must contain a policy; remove it to disable the window")
	}
	open, next, err := Evaluate(a[PolicyAnnotation], now())
	if err != nil {
		return time.Minute, err
	}
	if !open {
		return next.Sub(now()), nil
	}
	switch x := obj.(type) {
	case *appsv1.Deployment:
		if x.Spec.Paused {
			return time.Minute, nil
		}
	case *appsv1.StatefulSet:
		if x.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType || (x.Spec.UpdateStrategy.RollingUpdate != nil && x.Spec.UpdateStrategy.RollingUpdate.Partition != nil && *x.Spec.UpdateStrategy.RollingUpdate.Partition > 0) {
			return time.Minute, fmt.Errorf("StatefulSet requires unpartitioned RollingUpdate")
		}
	case *appsv1.DaemonSet:
		if x.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
			return time.Minute, fmt.Errorf("DaemonSet requires RollingUpdate")
		}
	}
	changed := false
	for _, s := range pending {
		if !allowedSource(s.Type) {
			continue
		}
		config, e := sourceConfig(ctx, c, t.Namespace, s)
		if e != nil {
			return time.Minute, e
		} // missing certificate: hold, never restart into missing Secret
		if !selector.Matches(labels.Set(config.Labels)) {
			continue
		}
		match := common.ShouldReload(config, t.Kind, a, template(obj).Annotations, common.GetCommandLineOptions())
		if !match.ShouldReload || (match.AutoReload && !references(&template(obj).Spec, s)) {
			continue
		}
		if applied[key(s)] != config.SHAValue {
			changed = true
			applied[key(s)] = config.SHAValue
		}
	}
	if len(applied) > 128 {
		return time.Minute, fmt.Errorf("too many applied sources; prune obsolete applied entries")
	}
	// Recheck immediately before mutation: source reads may have crossed the end.
	open, _, err = Evaluate(a[PolicyAnnotation], now())
	if err != nil || !open {
		return time.Minute, err
	}
	token := ""
	if changed {
		sum := sha256.Sum256([]byte(encode(applied)))
		token = hex.EncodeToString(sum[:])
	}
	err = patch(ctx, c, t, obj, map[string]any{PendingAnnotation: nil, AppliedAnnotation: encode(applied)}, token)
	if err != nil {
		return time.Minute, err
	}
	if changed && recorder != nil {
		recorder.Event(obj, corev1.EventTypeNormal, "WindowReloaded", "Applied pending configuration changes within restart window")
	}
	return 0, nil
}
