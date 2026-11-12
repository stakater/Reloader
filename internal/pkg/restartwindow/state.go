package restartwindow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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
	"github.com/stakater/Reloader/internal/pkg/options"
	"github.com/stakater/Reloader/internal/pkg/util"
	"github.com/stakater/Reloader/pkg/common"
)

type Source struct {
	Type         string `json:"type"`
	Name         string `json:"name"`
	ObservedHash string `json:"observedHash"`
}
type Target struct{ Kind, Namespace, Name string }
type State map[string]Source

type Result struct {
	RequeueAfter time.Duration
	Attempted    bool
	Executed     bool
}

type envMutation struct {
	Container string
	Name      string
	Value     string
}

func Supported(kind string) bool {
	return kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet"
}
func HasPolicy(a map[string]string) bool {
	_, ok := a[PolicyAnnotation]
	return ok
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
func patch(ctx context.Context, c kubernetes.Interface, t Target, obj runtime.Object, annotations map[string]any, token, attribution string, envMutations []envMutation) error {
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	body := map[string]any{"metadata": map[string]any{"uid": m.GetUID(), "resourceVersion": m.GetResourceVersion(), "annotations": annotations}}
	if token != "" || attribution != "" || len(envMutations) > 0 {
		templatePatch := map[string]any{}
		podAnnotations := map[string]string{}
		if token != "" {
			podAnnotations[RestartAnnotation] = token
		}
		if attribution != "" {
			podAnnotations[constants.ReloaderAnnotationPrefix+"/"+constants.LastReloadedFromAnnotation] = attribution
		}
		if len(podAnnotations) > 0 {
			templatePatch["metadata"] = map[string]any{"annotations": podAnnotations}
		}
		if len(envMutations) > 0 {
			byContainer := map[string][]map[string]string{}
			for _, mutation := range envMutations {
				byContainer[mutation.Container] = append(byContainer[mutation.Container], map[string]string{"name": mutation.Name, "value": mutation.Value})
			}
			containerNames := make([]string, 0, len(byContainer))
			for name := range byContainer {
				containerNames = append(containerNames, name)
			}
			sort.Strings(containerNames)
			containers := make([]map[string]any, 0, len(containerNames))
			for _, name := range containerNames {
				containers = append(containers, map[string]any{"name": name, "env": byContainer[name]})
			}
			templatePatch["spec"] = map[string]any{"containers": containers}
		}
		body["spec"] = map[string]any{"template": templatePatch}
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	switch t.Kind {
	case "Deployment":
		_, err = c.AppsV1().Deployments(t.Namespace).Patch(ctx, t.Name, types.StrategicMergePatchType, b, metav1.PatchOptions{})
	case "StatefulSet":
		_, err = c.AppsV1().StatefulSets(t.Namespace).Patch(ctx, t.Name, types.StrategicMergePatchType, b, metav1.PatchOptions{})
	case "DaemonSet":
		_, err = c.AppsV1().DaemonSets(t.Namespace).Patch(ctx, t.Name, types.StrategicMergePatchType, b, metav1.PatchOptions{})
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
		p, applied, err := state(m.GetAnnotations())
		if err != nil {
			return err
		}
		if applied[key(s)] == s.ObservedHash {
			if _, exists := p[key(s)]; !exists {
				return nil
			}
			delete(p, key(s))
			var pending any = encode(p)
			if len(p) == 0 {
				pending = nil
			}
			return patch(ctx, c, t, obj, map[string]any{PendingAnnotation: pending}, "", "", nil)
		}
		if old, ok := p[key(s)]; ok && old == s {
			return nil
		}
		if len(p) >= 128 {
			return fmt.Errorf("too many pending sources")
		}
		p[key(s)] = s
		return patch(ctx, c, t, obj, map[string]any{PendingAnnotation: encode(p)}, "", "", nil)
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
func containerUsingSource(p *corev1.PodSpec, s Source, autoReload bool) string {
	if len(p.Containers) == 0 {
		return ""
	}
	if !autoReload {
		return p.Containers[0].Name
	}
	volumeName := ""
	for _, v := range p.Volumes {
		if s.Type == constants.SecretEnvVarPostfix && v.Secret != nil && v.Secret.SecretName == s.Name {
			volumeName = v.Name
		}
		if s.Type == constants.ConfigmapEnvVarPostfix && v.ConfigMap != nil && v.ConfigMap.Name == s.Name {
			volumeName = v.Name
		}
		if v.Projected != nil {
			for _, x := range v.Projected.Sources {
				if s.Type == constants.SecretEnvVarPostfix && x.Secret != nil && x.Secret.Name == s.Name {
					volumeName = v.Name
				}
				if s.Type == constants.ConfigmapEnvVarPostfix && x.ConfigMap != nil && x.ConfigMap.Name == s.Name {
					volumeName = v.Name
				}
			}
		}
	}
	if volumeName != "" {
		for _, c := range p.Containers {
			for _, mount := range c.VolumeMounts {
				if mount.Name == volumeName {
					return c.Name
				}
			}
		}
		for _, c := range p.InitContainers {
			for _, mount := range c.VolumeMounts {
				if mount.Name == volumeName {
					return p.Containers[0].Name
				}
			}
		}
	}
	for _, c := range p.Containers {
		for _, e := range c.EnvFrom {
			if s.Type == constants.SecretEnvVarPostfix && e.SecretRef != nil && e.SecretRef.Name == s.Name {
				return c.Name
			}
			if s.Type == constants.ConfigmapEnvVarPostfix && e.ConfigMapRef != nil && e.ConfigMapRef.Name == s.Name {
				return c.Name
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil {
				if s.Type == constants.SecretEnvVarPostfix && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == s.Name {
					return c.Name
				}
				if s.Type == constants.ConfigmapEnvVarPostfix && e.ValueFrom.ConfigMapKeyRef != nil && e.ValueFrom.ConfigMapKeyRef.Name == s.Name {
					return c.Name
				}
			}
		}
	}
	for _, c := range p.InitContainers {
		for _, e := range c.EnvFrom {
			if s.Type == constants.SecretEnvVarPostfix && e.SecretRef != nil && e.SecretRef.Name == s.Name || s.Type == constants.ConfigmapEnvVarPostfix && e.ConfigMapRef != nil && e.ConfigMapRef.Name == s.Name {
				return p.Containers[0].Name
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil && (s.Type == constants.SecretEnvVarPostfix && e.ValueFrom.SecretKeyRef != nil && e.ValueFrom.SecretKeyRef.Name == s.Name || s.Type == constants.ConfigmapEnvVarPostfix && e.ValueFrom.ConfigMapKeyRef != nil && e.ValueFrom.ConfigMapKeyRef.Name == s.Name) {
				return p.Containers[0].Name
			}
		}
	}
	return ""
}

func containerEnvValue(p *corev1.PodSpec, containerName, envName string) (string, bool) {
	for _, container := range p.Containers {
		if container.Name != containerName {
			continue
		}
		for _, env := range container.Env {
			if env.Name == envName {
				return env.Value, true
			}
		}
	}
	return "", false
}

// Preflight evaluates persisted state from an informer-cached workload. Closed
// windows do not need a live API read; informer updates enqueue policy changes.
func Preflight(obj runtime.Object, now time.Time) (time.Duration, bool, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return 0, false, err
	}
	pending, _, err := state(m.GetAnnotations())
	if err != nil {
		return 0, false, err
	}
	if len(pending) == 0 {
		return 0, false, nil
	}
	open, next, err := Evaluate(m.GetAnnotations()[PolicyAnnotation], now)
	if err != nil {
		return 0, true, err
	}
	if open {
		return 0, true, nil
	}
	return next.Sub(now), true, nil
}

// Reconcile consumes all pending changes in one atomic template + metadata patch.
// A lost response or restarted leader cannot repeat an already committed restart.
func Reconcile(ctx context.Context, c kubernetes.Interface, t Target, now func() time.Time, selector labels.Selector, allowedSource func(string) bool, recorder record.EventRecorder) (Result, error) {
	obj, err := get(ctx, c, t)
	if apierrors.IsNotFound(err) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}
	m, _ := meta.Accessor(obj)
	a := m.GetAnnotations()
	pending, applied, err := state(a)
	if err != nil {
		return Result{}, err
	}
	if len(pending) == 0 {
		return Result{}, nil
	}
	if raw, present := a[PolicyAnnotation]; present && raw == "" {
		return Result{}, fmt.Errorf("restart-window annotation must contain a policy; remove it to disable the window")
	}
	currentTime := now()
	open, next, err := Evaluate(a[PolicyAnnotation], currentTime)
	if err != nil {
		return Result{}, err
	}
	if !open {
		return Result{RequeueAfter: next.Sub(currentTime)}, nil
	}
	switch x := obj.(type) {
	case *appsv1.Deployment:
		if x.Spec.Paused {
			return Result{}, nil
		}
	case *appsv1.StatefulSet:
		if x.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType || (x.Spec.UpdateStrategy.RollingUpdate != nil && x.Spec.UpdateStrategy.RollingUpdate.Partition != nil && *x.Spec.UpdateStrategy.RollingUpdate.Partition > 0) {
			return Result{}, fmt.Errorf("StatefulSet requires unpartitioned RollingUpdate")
		}
	case *appsv1.DaemonSet:
		if x.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType {
			return Result{}, fmt.Errorf("DaemonSet requires RollingUpdate")
		}
	}
	changed := false
	keys := make([]string, 0, len(pending))
	for pendingKey := range pending {
		keys = append(keys, pendingKey)
	}
	sort.Strings(keys)
	envMutations := make([]envMutation, 0, len(keys))
	attribution := ""
	markerChanged := false
	for _, pendingKey := range keys {
		s := pending[pendingKey]
		if !allowedSource(s.Type) {
			continue
		}
		config, e := sourceConfig(ctx, c, t.Namespace, s)
		if e != nil {
			return Result{}, e
		} // missing certificate: hold, never restart into missing Secret
		if !selector.Matches(labels.Set(config.Labels)) {
			continue
		}
		match := common.ShouldReload(config, t.Kind, a, template(obj).Annotations, common.GetCommandLineOptions())
		containerName := containerUsingSource(&template(obj).Spec, s, match.AutoReload)
		if !match.ShouldReload || containerName == "" {
			continue
		}
		if applied[key(s)] != config.SHAValue {
			changed = true
			applied[key(s)] = config.SHAValue
			if options.ReloadStrategy == constants.AnnotationsReloadStrategy {
				reloadSource := common.NewReloadSourceFromConfig(config, []string{containerName})
				attributionBytes, marshalErr := json.Marshal(reloadSource)
				if marshalErr != nil {
					return Result{}, marshalErr
				}
				attribution = string(attributionBytes)
			}
			if options.ReloadStrategy != constants.AnnotationsReloadStrategy {
				envName := constants.EnvVarPrefix + util.ConvertToEnvVarName(s.Name) + "_" + s.Type
				if value, ok := containerEnvValue(&template(obj).Spec, containerName, envName); !ok || value != config.SHAValue {
					markerChanged = true
					envMutations = append(envMutations, envMutation{Container: containerName, Name: envName, Value: config.SHAValue})
				}
			}
		}
	}
	if len(applied) > 128 {
		return Result{}, fmt.Errorf("too many applied sources; prune obsolete applied entries")
	}
	// Recheck immediately before mutation: source reads may have crossed the end.
	open, _, err = Evaluate(a[PolicyAnnotation], now())
	if err != nil || !open {
		return Result{}, err
	}
	token := ""
	executed := changed || markerChanged
	if executed {
		sum := sha256.Sum256([]byte(encode(applied)))
		token = hex.EncodeToString(sum[:])
	}
	err = patch(ctx, c, t, obj, map[string]any{PendingAnnotation: nil, AppliedAnnotation: encode(applied)}, token, attribution, envMutations)
	if err != nil {
		return Result{Attempted: true}, err
	}
	if executed && recorder != nil {
		recorder.Event(obj, corev1.EventTypeNormal, "WindowReloaded", "Applied pending configuration changes within restart window")
	}
	return Result{Attempted: executed, Executed: executed}, nil
}
