{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}

{{- define "reloader-name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" | lower -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "reloader-fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "reloader-chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "reloader-match-labels.chart" -}}
app: {{ template "reloader-fullname" . }}
release: {{ .Release.Name | quote }}
{{- end -}}

{{- define "reloader-labels.chart" -}}
{{ include "reloader-match-labels.chart" . }}
app.kubernetes.io/name: {{ template "reloader-name" . }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
heritage: {{ .Release.Service | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end -}}

{{/*
Create pod anti affinity labels
*/}}
{{- define "reloader-podAntiAffinity" -}}
podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    podAffinityTerm:
      labelSelector:
        matchExpressions:
        - key: app.kubernetes.io/instance
          operator: In
          values:
          - {{ .Release.Name | quote }}
      topologyKey: "kubernetes.io/hostname"
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "reloader-serviceAccountName" -}}
{{- if .Values.reloader.serviceAccount.create -}}
    {{ default (include "reloader-fullname" .) .Values.reloader.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.reloader.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
Create the annotations to support helm3
*/}}
{{- define "reloader-helm3.annotations" -}}
meta.helm.sh/release-namespace: {{ .Release.Namespace | quote }}
meta.helm.sh/release-name: {{ .Release.Name | quote }}
{{- end -}}

{{/*
Create the namespace selector if it does not watch globally
*/}}
{{- define "reloader-namespaceSelector" -}}
{{- if and .Values.reloader.watchGlobally .Values.reloader.namespaceSelector -}}
    {{ .Values.reloader.namespaceSelector }}
{{- end -}}
{{- end -}}

{{/*
Namespaces to watch in scoped mode: exactly the user-supplied reloader.namespaces,
trimmed, de-duped and sorted. The release namespace is intentionally NOT added here
— Reloader watches only what the user asked for. An empty result is not necessarily
global mode: with watchGlobally=false it becomes single-namespace mode (the release
namespace, injected via --namespaces by reloader-effectiveNamespaces-csv); only with
watchGlobally=true does empty mean watch-all.
Returns a JSON-encoded list; consumers use mustFromJson to iterate.
*/}}
{{- define "reloader-watchNamespaces" -}}
{{- $ns := .Values.reloader.namespaces | default list -}}
{{- if kindIs "string" $ns -}}
{{- $ns = splitList "," $ns -}}
{{- end -}}
{{- $clean := list -}}
{{- range $ns -}}
{{- $t := . | toString | trim -}}
{{- if $t -}}
{{- $clean = append $clean $t -}}
{{- end -}}
{{- end -}}
{{- $clean | uniq | sortAlpha | toJson -}}
{{- end -}}

{{/*
Comma-joined form of reloader-watchNamespaces, for the --namespaces CLI flag.
*/}}
{{- define "reloader-watchNamespaces-csv" -}}
{{- include "reloader-watchNamespaces" . | mustFromJson | join "," -}}
{{- end -}}

{{/*
The effective watched namespaces for the --namespaces CLI flag — the single
chart-side source of truth for watch scope, so the binary never has to fall back
to the KUBERNETES_NAMESPACE env:
  - scoped mode           -> the cleaned reloader.namespaces list
  - single-namespace mode -> the release namespace (watchGlobally=false, no list)
  - global mode           -> empty (no --namespaces flag; watch all)
Returns a comma-joined string ("" in global mode).
*/}}
{{- define "reloader-effectiveNamespaces-csv" -}}
{{- $watch := include "reloader-watchNamespaces" . | mustFromJson -}}
{{- if $watch -}}
{{- $watch | join "," -}}
{{- else if not .Values.reloader.watchGlobally -}}
{{- .Values.namespace | default .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{/*
Whether Reloader runs in scoped mode. This is the single source of truth for the
scoped-vs-global decision: it is true only when the cleaned watch list
(reloader-watchNamespaces) is non-empty. Gate on this rather than the raw
.Values.reloader.namespaces, which is truthy even for values like " , " that trim
to an empty list (those must fall through to global/single-namespace mode).
Returns "true" (truthy) or "" (falsy).
*/}}
{{- define "reloader-isScoped" -}}
{{- if include "reloader-watchNamespaces" . | mustFromJson -}}
true
{{- end -}}
{{- end -}}

{{/*
Fails the render on an inconsistent namespace configuration: reloader.namespaces
(scoped mode) requires reloader.watchGlobally=false. Included from deployment.yaml
so it is validated once regardless of which templates render.
*/}}
{{- define "reloader-validate-namespaces" -}}
{{- if and .Values.reloader.watchGlobally (include "reloader-isScoped" .) -}}
{{- fail "reloader.namespaces is set but reloader.watchGlobally is true; set reloader.watchGlobally=false to use scoped namespace mode." -}}
{{- end -}}
{{- end -}}

{{/*
RBAC rules Reloader needs in its own (release) namespace, independent of the
watched namespaces. Reloader publishes an internal meta-info ConfigMap there in
every mode, so configmap write access is always granted. In scoped mode the
release namespace is not covered by the watch RBAC, so under HA the leader-election
leases and the events it emits are granted here too; in global/single mode those
are already covered by the ClusterRole or the single-namespace Role.
Expects the root context ($) as its argument.
*/}}
{{- define "reloader-release-rules" }}
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - create
      - update
      - patch
{{- if and (include "reloader-isScoped" .) .Values.reloader.enableHA }}
  - apiGroups:
      - "coordination.k8s.io"
    resources:
      - leases
    verbs:
      - create
      - get
      - update
  - apiGroups:
      - ""
      - "events.k8s.io"
    resources:
      - events
    verbs:
      - create
      - patch
      - update
{{- end }}
{{- end -}}

{{/*
The namespaced RBAC rules granted to Reloader in every watched namespace.
Shared between the single-namespace Role and the per-namespace scoped Roles so
the rule set is defined once. Expects the root context ($) as its argument.
*/}}
{{- define "reloader-namespaced-rules" }}
  - apiGroups:
      - ""
    resources:
{{- if .Values.reloader.ignoreSecrets }}{{- else }}
      - secrets
{{- end }}
{{- if .Values.reloader.ignoreConfigMaps }}{{- else }}
      - configmaps
{{- end }}
    verbs:
      - list
      - get
      - watch
{{- if and (.Capabilities.APIVersions.Has "apps.openshift.io/v1") (.Values.reloader.isOpenshift) }}
  - apiGroups:
      - "apps.openshift.io"
      - ""
    resources:
      - deploymentconfigs
    verbs:
      - list
      - get
      - update
      - patch
{{- end }}
{{- if and (.Capabilities.APIVersions.Has "argoproj.io/v1alpha1") (.Values.reloader.isArgoRollouts) }}
  - apiGroups:
      - "argoproj.io"
    resources:
      - rollouts
    verbs:
      - list
      - get
      - watch
      - update
      - patch
{{- end }}
  - apiGroups:
      - "apps"
    resources:
      - deployments
      - daemonsets
      - statefulsets
    verbs:
      - list
      - get
      - watch
      - update
      - patch
{{- if .Values.reloader.ignoreCronJobs }}{{- else }}
  - apiGroups:
      - "batch"
    resources:
      - cronjobs
    verbs:
      - list
      - get
      - watch
      - update
      - patch
{{- end }}
{{- if .Values.reloader.ignoreJobs }}{{- else }}
  - apiGroups:
      - "batch"
    resources:
      - jobs
    verbs:
      - create
      - delete
      - list
      - get
      - watch
{{- end }}
{{- if .Values.reloader.enableHA }}
  - apiGroups:
      - "coordination.k8s.io"
    resources:
      - leases
    verbs:
      - create
      - get
      - update
{{- end}}
{{- if .Values.reloader.enableCSIIntegration }}
  - apiGroups:
      - "secrets-store.csi.x-k8s.io"
    resources:
      - secretproviderclasspodstatuses
      - secretproviderclasses
    verbs:
      - list
      - get
      - watch
{{- end}}
  - apiGroups:
      - ""
      - "events.k8s.io"
    resources:
      - events
    verbs:
      - create
      - patch
      - update
{{- end -}}

{{/*
Normalizes global.imagePullSecrets to a list of objects with name fields.
Supports both of these in values.yaml:
  # - name: my-pull-secret
  # - my-pull-secret
*/}}
{{- define "reloader-imagePullSecrets" -}}
{{- range $s := .Values.global.imagePullSecrets }}
{{- if kindIs "map" $s }}
- {{ toYaml $s | nindent 2 | trim }}
{{- else }}
- name: {{ $s }}
{{- end }}
{{- end }}
{{- end -}}
