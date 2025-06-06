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
app.kubernetes.io/name: {{ template "reloader-name" . }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
{{- end -}}

{{- define "reloader-labels.chart" -}}
{{ include "reloader-match-labels.chart" . }}
helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"
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
