{{/*
driver fullname
*/}}
{{- define "juicefs-csi.driverFullname" -}}
{{ include "juicefs-csi.fullname" . }}-driver
{{- end }}

{{/*
driver common labels
*/}}
{{- define "juicefs-csi.driverLabels" -}}
{{ include "juicefs-csi.labels" . }}
app.kubernetes.io/component: driver
{{- end }}

{{/*
driver selector labels
*/}}
{{- define "juicefs-csi.driverSelectorLabels" -}}
{{ include "juicefs-csi.selectorLabels" . }}
app.kubernetes.io/component: driver
{{- end }}

{{/*
Docker image name for driver
*/}}
{{- define "juicefs-csi.driverImage" -}}
{{- $registry := coalesce .Values.driver.registry "docker.io" -}}
{{- $repository := coalesce .Values.driver.repository -}}
{{- $tag := coalesce .Values.driver.tag "latest" -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- end -}}