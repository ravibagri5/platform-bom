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

{{- define "platform-bom.customResource" -}}
{{- $resource := .resource -}}
{{- $metadata := required "custom resource metadata is required" $resource.metadata -}}
{{- $annotations := default dict $metadata.annotations -}}
{{- if $metadata.displayName -}}
{{- $_ := set $annotations "pbom.dev/display-name" $metadata.displayName -}}
{{- end -}}
apiVersion: pbom.dev/v1alpha1
kind: {{ required "custom resource kind is required" $resource.kind }}
metadata:
  name: {{ required "custom resource metadata.name is required" $metadata.name | quote }}
  namespace: {{ .root.Release.Namespace | quote }}
  {{- with $metadata.labels }}
  labels:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  {{- with $annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  {{- toYaml (required "custom resource spec is required" $resource.spec) | nindent 2 }}
{{- end -}}