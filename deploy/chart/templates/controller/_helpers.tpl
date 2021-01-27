{{/*
controller fullname
*/}}
{{- define "juicefs-csi.controllerFullname" -}}
{{ include "juicefs-csi.fullname" . }}-controller
{{- end }}

{{/*
controller common labels
*/}}
{{- define "juicefs-csi.controllerLabels" -}}
{{ include "juicefs-csi.labels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
controller selector labels
*/}}
{{- define "juicefs-csi.controllerSelectorLabels" -}}
{{ include "juicefs-csi.selectorLabels" . }}
app.kubernetes.io/component: controller
{{- end }}

{{/*
attacherRole fullname
*/}}
{{- define "juicefs-csi.attacherRole" -}}
{{ include "juicefs-csi.fullname" . }}-attach-role
{{- end }}

{{/*
attacherRoleBinding fullname
*/}}
{{- define "juicefs-csi.attacherRoleBinding" -}}
{{ include "juicefs-csi.fullname" . }}-attach-role-binding
{{- end }}

{{/*
provisionerRole fullname
*/}}
{{- define "juicefs-csi.provisionerRole" -}}
{{ include "juicefs-csi.fullname" . }}-provisioner
{{- end }}

{{/*
provisionerRoleBinding fullname
*/}}
{{- define "juicefs-csi.provisionerRoleBinding" -}}
{{ include "juicefs-csi.fullname" . }}-provisioner-binding
{{- end }}

{{/*
Docker image name for controller
*/}}
{{- define "juicefs-csi.controllerImage" -}}
{{- $registry := coalesce .Values.controller.registry "docker.io" -}}
{{- $repository := coalesce .Values.controller.repository -}}
{{- $tag := coalesce .Values.controller.tag "latest" -}}
{{- printf "%s/%s:%s" $registry $repository $tag -}}
{{- end -}}