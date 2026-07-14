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
— Reloader watches only what the user asked for (an empty result means global mode).
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
Namespaces that need namespaced RBAC in scoped mode: the watched namespaces plus
the release namespace, so leader-election leases, the meta-info ConfigMap and
events keep working there even though it is not watched for reloads.
Returns a JSON-encoded list; consumers use mustFromJson to iterate.
*/}}
{{- define "reloader-rbacNamespaces" -}}
{{- $relNs := .Values.namespace | default .Release.Namespace -}}
{{- $watch := include "reloader-watchNamespaces" . | mustFromJson -}}
{{- concat (list $relNs) $watch | uniq | sortAlpha | toJson -}}
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
      - ""
    resources:
      - rollouts
    verbs:
      - list
      - get
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
      - update
      - patch
  - apiGroups:
      - "batch"
    resources:
      - cronjobs
    verbs:
      - list
      - get
  - apiGroups:
      - "batch"
    resources:
      - jobs
    verbs:
      - create
      - delete
      - list
      - get
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
    resources:
      - events
    verbs:
      - create
      - patch
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
