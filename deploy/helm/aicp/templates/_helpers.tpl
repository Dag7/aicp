{{/*
Expand the name of the chart.
*/}}
{{- define "aicp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aicp.fullname" -}}
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
{{- define "aicp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to all resources.
*/}}
{{- define "aicp.labels" -}}
helm.sh/chart: {{ include "aicp.chart" . }}
{{ include "aicp.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels used for matching pods to services and deployments.
*/}}
{{- define "aicp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aicp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Validator labels.
*/}}
{{- define "aicp.validatorLabels" -}}
helm.sh/chart: {{ include "aicp.chart" . }}
{{ include "aicp.validatorSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Validator selector labels.
*/}}
{{- define "aicp.validatorSelectorLabels" -}}
app.kubernetes.io/name: {{ include "aicp.name" . }}-validator
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: validator
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "aicp.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "aicp.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the PostgreSQL connection URL.
*/}}
{{- define "aicp.databaseUrl" -}}
{{- if .Values.postgresql.internal }}
{{- printf "postgres://%s:%s@%s-postgresql:5432/%s?sslmode=disable" .Values.postgresql.auth.username .Values.postgresql.auth.password (include "aicp.fullname" .) .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.postgresql.externalUrl }}
{{- end }}
{{- end }}

{{/*
Return the Redis connection URL.
*/}}
{{- define "aicp.redisUrl" -}}
{{- if .Values.redis.internal }}
{{- printf "redis://%s-redis-master:6379/0" (include "aicp.fullname" .) }}
{{- else }}
{{- .Values.redis.externalUrl }}
{{- end }}
{{- end }}
