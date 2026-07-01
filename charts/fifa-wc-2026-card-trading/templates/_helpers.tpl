{{- define "fifa-wc-2026-card-trading.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "fifa-wc-2026-card-trading.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "fifa-wc-2026-card-trading.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "fifa-wc-2026-card-trading.labels" -}}
app.kubernetes.io/name: {{ include "fifa-wc-2026-card-trading.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "fifa-wc-2026-card-trading.selectorLabels" -}}
app.kubernetes.io/name: {{ include "fifa-wc-2026-card-trading.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "fifa-wc-2026-card-trading.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "fifa-wc-2026-card-trading.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "fifa-wc-2026-card-trading.secretName" -}}
{{- if .Values.secrets.existingSecret -}}
{{- .Values.secrets.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "fifa-wc-2026-card-trading.fullname" .) -}}
{{- end -}}
{{- end -}}
