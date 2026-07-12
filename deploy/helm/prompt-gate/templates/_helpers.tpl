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

{{/*
Existing runtime secret name.
*/}}
{{- define "prompt-gate.secretName" -}}
{{- default (printf "%s-secrets" (include "prompt-gate.fullname" .)) .Values.secret.existingSecret -}}
{{- end -}}

{{/*
Trim the configured inline administration API key. The application applies the
same normalization when loading PROMPTGATE_ADMIN_API_KEY.
*/}}
{{- define "prompt-gate.adminApiKeyValue" -}}
{{- default "" .Values.adminApiKey.value | toString | trim -}}
{{- end -}}

{{/*
Normalized existing administration API key Secret name and key.
*/}}
{{- define "prompt-gate.adminApiKeyExistingSecretName" -}}
{{- default "" .Values.adminApiKey.existingSecret.name | toString | trim -}}
{{- end -}}

{{- define "prompt-gate.adminApiKeyExistingSecretKey" -}}
{{- default "" .Values.adminApiKey.existingSecret.key | toString | trim -}}
{{- end -}}

{{/*
Validate that the administration API key has at most one source and that an
existing Secret reference is complete.
*/}}
{{- define "prompt-gate.validateAdminApiKey" -}}
{{- $value := include "prompt-gate.adminApiKeyValue" . -}}
{{- $existingSecretName := include "prompt-gate.adminApiKeyExistingSecretName" . -}}
{{- $existingSecretKey := include "prompt-gate.adminApiKeyExistingSecretKey" . -}}
{{- $runtimeSecretName := include "prompt-gate.secretName" . | trim -}}
{{- $adminSecretName := include "prompt-gate.adminApiKeySecretName" . | trim -}}
{{- if and (ne $value "") (ne $existingSecretName "") -}}
{{- fail "adminApiKey.value and adminApiKey.existingSecret.name are mutually exclusive" -}}
{{- end -}}
{{- if and (ne $existingSecretName "") (eq $existingSecretKey "") -}}
{{- fail "adminApiKey.existingSecret.key is required when adminApiKey.existingSecret.name is set" -}}
{{- end -}}
{{- if and (or (ne $value "") (ne $existingSecretName "")) (eq $adminSecretName $runtimeSecretName) -}}
{{- fail "the administration API key Secret name must differ from the shared runtime Secret name" -}}
{{- end -}}
{{- end -}}

{{/*
Administration API key Secret name. Use the external name when configured,
otherwise use the name of the chart-managed dedicated Secret.
*/}}
{{- define "prompt-gate.adminApiKeySecretName" -}}
{{- $existingSecretName := include "prompt-gate.adminApiKeyExistingSecretName" . -}}
{{- if ne $existingSecretName "" -}}
{{- $existingSecretName -}}
{{- else -}}
{{- $fullname := include "prompt-gate.fullname" . -}}
{{- if le (len $fullname) 49 -}}
{{- printf "%s-admin-api-key" $fullname -}}
{{- else -}}
{{- $prefix := $fullname | trunc 36 | trimSuffix "-" -}}
{{- $hash := $fullname | sha256sum | trunc 12 -}}
{{- printf "%s-%s-admin-api-key" $prefix $hash -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Administration API key Secret data key.
*/}}
{{- define "prompt-gate.adminApiKeySecretKey" -}}
{{- $existingSecretName := include "prompt-gate.adminApiKeyExistingSecretName" . -}}
{{- if ne $existingSecretName "" -}}
{{- include "prompt-gate.adminApiKeyExistingSecretKey" . -}}
{{- else -}}
PROMPTGATE_ADMIN_API_KEY
{{- end -}}
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
{{- default (printf "%s://%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host) .Values.config.frontendBaseUrl -}}
{{- end -}}

{{/*
Public backend URL.
*/}}
{{- define "prompt-gate.backendBaseUrl" -}}
{{- default (printf "%s://%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host) .Values.config.backendBaseUrl -}}
{{- end -}}

{{/*
Public proxy URL.
*/}}
{{- define "prompt-gate.proxyBaseUrl" -}}
{{- default (printf "%s://%s%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host .Values.ingress.proxyPath) .Values.config.proxyBaseUrl -}}
{{- end -}}

{{/*
Kyverno-compatible pod security context.
*/}}
{{- define "prompt-gate.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 1000
runAsGroup: 1000
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

{{/*
Shared envFrom block.
*/}}
{{- define "prompt-gate.envFrom" -}}
envFrom:
  - configMapRef:
      name: {{ include "prompt-gate.fullname" . }}
  - secretRef:
      name: {{ include "prompt-gate.secretName" . }}
      optional: false
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
