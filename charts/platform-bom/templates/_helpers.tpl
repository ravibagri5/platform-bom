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

{{/* Releases installed by an older chart and upgraded with --reuse-values have no configMaps values. */}}
{{- define "platform-bom.createConfigMaps" -}}
{{- $cm := default dict .Values.configMaps -}}
{{- if or (not (hasKey $cm "create")) $cm.create -}}true{{- end -}}
{{- end -}}

{{- define "platform-bom.configMapName" -}}
{{- if include "platform-bom.createConfigMaps" .root -}}
{{- printf "%s-%s" (include "platform-bom.fullname" .root) .kind -}}
{{- else -}}
{{- $cm := default dict .root.Values.configMaps -}}
{{- default (printf "pbom-%s" .kind) (index $cm .kind) -}}
{{- end -}}
{{- end -}}

{{/* ConfigMap data from a map of file name to document (a map) or raw file content (a string). */}}
{{- define "platform-bom.files" -}}
{{- range $name, $doc := .files }}
{{- if not (regexMatch "^[A-Za-z0-9][A-Za-z0-9._-]*\\.ya?ml$" $name) }}
{{- fail (printf "%s key %q must be a file name ending in .yaml or .yml" $.value $name) }}
{{- end }}
{{ $name | quote }}: |
  {{- if kindIs "string" $doc }}
  {{- $doc | nindent 2 }}
  {{- else }}
  {{- toYaml $doc | nindent 2 }}
  {{- end }}
{{- end }}
{{- end -}}