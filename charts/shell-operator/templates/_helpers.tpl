{{- define "shell-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shell-operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "shell-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shell-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "shell-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "shell-operator.labels" -}}
helm.sh/chart: {{ include "shell-operator.chart" . }}
{{ include "shell-operator.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{- define "shell-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "shell-operator.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "shell-operator.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
{{- end -}}

{{- define "shell-operator.hooksConfigMapName" -}}
{{- default (printf "%s-hooks" (include "shell-operator.fullname" .)) .Values.hooks.existingConfigMap -}}
{{- end -}}

{{- define "shell-operator.validatingServiceName" -}}
{{- printf "%s-validating" (include "shell-operator.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shell-operator.conversionServiceName" -}}
{{- printf "%s-conversion" (include "shell-operator.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "shell-operator.rules" -}}
{{- $rules := list -}}
{{- range .Values.rbac.rules -}}
{{- $rules = append $rules . -}}
{{- end -}}
{{- if .Values.webhooks.validating.enabled -}}
{{- $rules = append $rules (dict "apiGroups" (list "admissionregistration.k8s.io") "resources" (list "validatingwebhookconfigurations") "verbs" (list "create" "list" "update")) -}}
{{- end -}}
{{- if .Values.webhooks.conversion.enabled -}}
{{- $rules = append $rules (dict "apiGroups" (list "apiextensions.k8s.io") "resources" (list "customresourcedefinitions") "verbs" (list "get" "list" "update")) -}}
{{- end -}}
{{- toYaml $rules -}}
{{- end -}}

{{- define "shell-operator.validate" -}}
{{- if and .Values.hooks.files .Values.hooks.existingConfigMap -}}
{{- fail "hooks.files and hooks.existingConfigMap are mutually exclusive" -}}
{{- end -}}
{{- if and .Values.webhooks.validating.enabled (not .Values.webhooks.validating.certSecretName) -}}
{{- fail "webhooks.validating.enabled requires webhooks.validating.certSecretName" -}}
{{- end -}}
{{- if and .Values.webhooks.conversion.enabled (not .Values.webhooks.conversion.certSecretName) -}}
{{- fail "webhooks.conversion.enabled requires webhooks.conversion.certSecretName" -}}
{{- end -}}
{{- end -}}
