{{/*
Expand the name of the chart.
*/}}
{{- define "juicefs-csi.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "juicefs-csi.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "juicefs-csi.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "juicefs-csi.labels" -}}
helm.sh/chart: {{ include "juicefs-csi.chart" . }}
{{ include "juicefs-csi.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "juicefs-csi.selectorLabels" -}}
app.kubernetes.io/name: {{ include "juicefs-csi.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "juicefs-csi.controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.controller.create }}
{{- default (include "juicefs-csi.fullname" .) .Values.serviceAccount.controller.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.controller.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "juicefs-csi.node.serviceAccountName" -}}
{{- if .Values.serviceAccount.node.create }}
{{- default (include "juicefs-csi.fullname" .) .Values.serviceAccount.node.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.node.name }}
{{- end }}
{{- end }}

{{/*
secret fullname
*/}}
{{- define "juicefs-csi.secretFullname" -}}
{{ include "juicefs-csi.fullname" . }}-secret
{{- end }}
