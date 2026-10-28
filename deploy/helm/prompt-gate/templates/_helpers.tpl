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

{{/*
Validated public proxy path. Keep the path unescaped and segment-safe so the
Ingress route and the application base path always have identical semantics.
*/}}
{{- define "prompt-gate.proxyPath" -}}
{{- $proxyPath := required "ingress.proxyPath must not be empty" .Values.ingress.proxyPath -}}
{{- if or (not (regexMatch "^/[A-Za-z0-9._~-]+(/[A-Za-z0-9._~-]+)*$" $proxyPath)) (ne (clean $proxyPath) $proxyPath) -}}
{{- fail "ingress.proxyPath must be a normalized absolute non-root path without a trailing slash, encoding, query, fragment, or dot segments" -}}
{{- end -}}
{{- $proxyPath -}}
{{- end -}}

{{/*
Return the normalized path component of an absolute public proxy URL. A root
path is equivalent to no application base path.
*/}}
{{- define "prompt-gate.publicProxyURLPath" -}}
{{- $rawURL := trim . -}}
{{- $parsedURL := urlParse $rawURL -}}
{{- $scheme := default "" (get $parsedURL "scheme") -}}
{{- $host := default "" (get $parsedURL "host") -}}
{{- $userinfo := default "" (get $parsedURL "userinfo") -}}
{{- $query := default "" (get $parsedURL "query") -}}
{{- $fragment := default "" (get $parsedURL "fragment") -}}
{{- if or (not (or (eq $scheme "http") (eq $scheme "https"))) (eq $host "") (ne $userinfo "") (ne $query "") (ne $fragment "") (contains "%" $rawURL) -}}
{{- fail "config.common.public_urls.proxy must be an absolute HTTP(S) URL without credentials, encoding, query, or fragment" -}}
{{- end -}}
{{- $urlPath := default "" (get $parsedURL "path") -}}
{{- if eq $urlPath "/" -}}
{{- $urlPath = "" -}}
{{- end -}}
{{- $urlPath -}}
{{- end -}}

{{/* Render the complete runtime document and derive public URLs from Ingress. */}}
{{- define "prompt-gate.runtimeConfig" -}}
{{- $config := deepCopy .Values.config -}}
{{- $environment := required "config.common.environment must not be empty" $config.common.environment -}}
{{- if not (has $environment (list "development" "staging" "production")) -}}
{{- fail "config.common.environment must be one of development, staging, or production" -}}
{{- end -}}
{{- $configuredBasePath := trim (default "" $config.proxy.base_path) -}}
{{- $configuredProxyURL := trim (default "" $config.common.public_urls.proxy) -}}
{{- $_ := set $config.proxy "base_path" $configuredBasePath -}}
{{- if and .Values.ingress.enabled .Values.proxy.enabled -}}
{{- $ingressProxyPath := include "prompt-gate.proxyPath" . -}}
{{- if and (ne $configuredBasePath "") (ne $configuredBasePath $ingressProxyPath) -}}
{{- fail "config.proxy.base_path must be empty or equal ingress.proxyPath when the proxy Ingress is enabled" -}}
{{- end -}}
{{- if ne $configuredProxyURL "" -}}
{{- $configuredURLPath := include "prompt-gate.publicProxyURLPath" $configuredProxyURL -}}
{{- if ne $configuredURLPath $ingressProxyPath -}}
{{- fail "config.common.public_urls.proxy path must equal ingress.proxyPath when the proxy Ingress is enabled" -}}
{{- end -}}
{{- else -}}
{{- $configuredProxyURL = printf "%s://%s%s" (include "prompt-gate.publicScheme" .) .Values.ingress.host $ingressProxyPath -}}
{{- end -}}
{{- $_ := set $config.proxy "base_path" $ingressProxyPath -}}
{{- else if .Values.proxy.enabled -}}
{{- if eq $configuredProxyURL "" -}}
{{- fail "config.common.public_urls.proxy is required when the chart-managed proxy Ingress is disabled" -}}
{{- end -}}
{{- $configuredURLPath := include "prompt-gate.publicProxyURLPath" $configuredProxyURL -}}
{{- if ne $configuredURLPath $configuredBasePath -}}
{{- fail "config.common.public_urls.proxy path must equal config.proxy.base_path when the chart-managed proxy Ingress is disabled" -}}
{{- end -}}
{{- end -}}
{{- $_ := set $config.common.public_urls "frontend" (include "prompt-gate.frontendBaseUrl" .) -}}
{{- $_ := set $config.common.public_urls "backend" (include "prompt-gate.backendBaseUrl" .) -}}
{{- $_ := set $config.common.public_urls "proxy" $configuredProxyURL -}}
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
