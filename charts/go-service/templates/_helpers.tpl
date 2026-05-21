{{- define "go-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "go-service.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "go-service.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

