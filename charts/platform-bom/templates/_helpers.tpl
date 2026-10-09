{{- define "platform-bom.fullname" -}}
{{- printf "%s-platform-bom" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "platform-bom.labels" -}}
app.kubernetes.io/name: platform-bom
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "platform-bom.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "platform-bom.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- required "serviceAccount.name is required when serviceAccount.create=false" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "platform-bom.rbacName" -}}
{{- printf "%s-%s" .Release.Namespace (include "platform-bom.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}