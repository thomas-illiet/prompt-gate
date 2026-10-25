{{/*
Expand the chart name.
*/}}
{{- define "prompt-gate.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "prompt-gate.fullname" -}}
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

{{/*
Create chart label.
*/}}
{{- define "prompt-gate.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "prompt-gate.labels" -}}
helm.sh/chart: {{ include "prompt-gate.chart" . }}
{{ include "prompt-gate.selectorLabels" . }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "prompt-gate.selectorLabels" -}}
app.kubernetes.io/name: {{ include "prompt-gate.name" . }}
prompt-gate.io/instance: {{ include "prompt-gate.fullname" . }}
{{- end -}}

{{/*
Service account name.
*/}}
{{- define "prompt-gate.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "prompt-gate.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Runtime image.
*/}}
{{- define "prompt-gate.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{/* Runtime configuration Secret name and key. */}}
{{- define "prompt-gate.configSecretName" -}}
{{- default (printf "%s-config" (include "prompt-gate.fullname" .)) .Values.configSecret.existingSecret -}}
{{- end -}}

{{- define "prompt-gate.configSecretKey" -}}
{{- required "configSecret.key must not be empty" .Values.configSecret.key -}}
{{- end -}}

{{/* Render the complete runtime document and derive public URLs from Ingress. */}}
{{- define "prompt-gate.runtimeConfig" -}}
{{- $config := deepCopy .Values.config -}}
{{- $_ := set $config.common.public_urls "frontend" (include "prompt-gate.frontendBaseUrl" .) -}}
{{- $_ := set $config.common.public_urls "backend" (include "prompt-gate.backendBaseUrl" .) -}}
{{- $_ := set $config.common.public_urls "proxy" (include "prompt-gate.proxyBaseUrl" .) -}}
{{- toYaml $config -}}
{{- end -}}

{{/*
Public URL scheme derived from Ingress TLS.
*/}}
{{- define "prompt-gate.publicScheme" -}}
{{- if .Values.ingress.tls.enabled -}}https{{- else -}}http{{- end -}}
{{- end -}}

{{/*
Public frontend URL.
*/}}
{{- define "prompt-gate.frontendBaseUrl" -}}
{{- default (printf "%s://%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host) .Values.config.common.public_urls.frontend -}}
{{- end -}}

{{/*
Public backend URL.
*/}}
{{- define "prompt-gate.backendBaseUrl" -}}
{{- default (printf "%s://%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host) .Values.config.common.public_urls.backend -}}
{{- end -}}

{{/*
Public proxy URL.
*/}}
{{- define "prompt-gate.proxyBaseUrl" -}}
{{- default (printf "%s://%s%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host .Values.ingress.proxyPath) .Values.config.common.public_urls.proxy -}}
{{- end -}}

{{/*
Kyverno-compatible pod security context.
*/}}
{{- define "prompt-gate.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 1000
runAsGroup: 1000
fsGroup: 1000
fsGroupChangePolicy: OnRootMismatch
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/*
Kyverno-compatible container security context.
*/}}
{{- define "prompt-gate.containerSecurityContext" -}}
allowPrivilegeEscalation: false
privileged: false
readOnlyRootFilesystem: true
runAsNonRoot: true
runAsUser: 1000
runAsGroup: 1000
capabilities:
  drop:
    - ALL
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{/* Shared read-only runtime configuration mount and volume. */}}
{{- define "prompt-gate.configVolumeMount" -}}
- name: promptgate-config
  mountPath: /etc/prompt-gate/promptgate.yaml
  subPath: promptgate.yaml
  readOnly: true
{{- end -}}

{{- define "prompt-gate.configVolume" -}}
- name: promptgate-config
  secret:
    secretName: {{ include "prompt-gate.configSecretName" . }}
    defaultMode: 0440
    items:
      - key: {{ include "prompt-gate.configSecretKey" . | quote }}
        path: promptgate.yaml
{{- end -}}

{{/*
Shared image pull secrets.
*/}}
{{- define "prompt-gate.imagePullSecrets" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}

{{/*
Shared pod placement.
*/}}
{{- define "prompt-gate.podPlacement" -}}
{{- with .Values.nodeSelector }}
nodeSelector:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.affinity }}
affinity:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.tolerations }}
tolerations:
{{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}
