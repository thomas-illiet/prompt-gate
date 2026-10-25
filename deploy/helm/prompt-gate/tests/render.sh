#!/usr/bin/env bash

set -euo pipefail

chart_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

required_values=(
  --set-string config.common.database.url=postgres://user:pass@db/promptgate
  --set-string config.common.redis.url=redis://redis:6379
  --set-string config.common.security.jwt_secret=abcdefghijklmnopqrstuvwxyz012345
  --set-string config.common.security.secrets_key=0123456789abcdef0123456789abcdef
)

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq -- "${expected}" "${file}"; then
    echo "expected ${file} to contain: ${expected}" >&2
    exit 1
  fi
}

assert_ingress_route() {
  local file="$1"
  local path="$2"
  local service="$3"
  if ! awk -v expected_path="${path}" -v expected_service="${service}" '
    $1 == "-" && $2 == "path:" {
      rendered_path = $3
      gsub(/^"|"$/, "", rendered_path)
      active = rendered_path == expected_path
      next
    }
    active && $1 == "name:" && $2 == expected_service { found = 1 }
    END { exit found ? 0 : 1 }
  ' "${file}"; then
    echo "expected Ingress route ${path} to use service ${service}" >&2
    exit 1
  fi
}

helm lint "${chart_dir}" --strict "${required_values[@]}"

default_render="${tmp_dir}/default.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" >"${default_render}"
assert_contains "${default_render}" 'kind: Secret'
assert_contains "${default_render}" '"promptgate.yaml": |'
assert_contains "${default_render}" 'database:'
assert_contains "${default_render}" 'url: postgres://user:pass@db/promptgate'
assert_contains "${default_render}" 'base_path: /remote'
assert_contains "${default_render}" 'proxy: https://promptgate.example.com/remote'
assert_contains "${default_render}" '/etc/prompt-gate/promptgate.yaml'
assert_contains "${default_render}" 'defaultMode: 0440'
if grep -Eq -- '^kind: ConfigMap$|envFrom:|PROMPTGATE_' "${default_render}"; then
  echo "runtime configuration must be mounted as YAML without PROMPTGATE_* environment variables" >&2
  exit 1
fi
if [[ "$(grep -c '^kind: Ingress$' "${default_render}")" -ne 1 ]]; then
  echo "expected the chart to render one Ingress object" >&2
  exit 1
fi
if grep -Fq -- 'haproxy.org/path-rewrite:' "${default_render}"; then
  echo "the chart must not render an HAProxy path rewrite" >&2
  exit 1
fi

ingress_render="${tmp_dir}/ingress.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --show-only templates/ingress.yaml >"${ingress_render}"
assert_contains "${ingress_render}" 'ingressClassName: "haproxy"'
assert_contains "${ingress_render}" 'haproxy.org/forwarded-for: "true"'
if [[ "$(grep -c '^[[:space:]]*- path:' "${ingress_render}")" -ne 2 ]]; then
  echo "expected exactly two routes in the default Ingress" >&2
  exit 1
fi
assert_ingress_route "${ingress_render}" '/remote' 'audit-prompt-gate-proxy'
assert_ingress_route "${ingress_render}" '/' 'audit-prompt-gate-api'

oidc_redirect_render="${tmp_dir}/oidc-redirect.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set ingress.tls.enabled=false \
  --set-string config.api.oidc.redirect_url=https://promptgate.example.com/auth/callback >"${oidc_redirect_render}"
assert_contains "${oidc_redirect_render}" 'backend: http://promptgate.example.com'
assert_contains "${oidc_redirect_render}" 'redirect_url: https://promptgate.example.com/auth/callback'

custom_path_render="${tmp_dir}/custom-path.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string ingress.proxyPath=/gateway/v1 >"${custom_path_render}"
assert_contains "${custom_path_render}" 'path: "/gateway/v1"'
assert_contains "${custom_path_render}" 'base_path: /gateway/v1'
assert_contains "${custom_path_render}" 'proxy: https://promptgate.example.com/gateway/v1'

base_path_mismatch="${tmp_dir}/base-path-mismatch.txt"
if helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string config.proxy.base_path=/other >"${base_path_mismatch}" 2>&1; then
  echo "expected config.proxy.base_path to match ingress.proxyPath" >&2
  exit 1
fi
assert_contains "${base_path_mismatch}" 'config.proxy.base_path must be empty or equal ingress.proxyPath'

public_url_mismatch="${tmp_dir}/public-url-mismatch.txt"
if helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string config.common.public_urls.proxy=https://proxy.example.test/other >"${public_url_mismatch}" 2>&1; then
  echo "expected public proxy URL path to match ingress.proxyPath" >&2
  exit 1
fi
assert_contains "${public_url_mismatch}" 'config.common.public_urls.proxy path must equal ingress.proxyPath'

public_url_override="${tmp_dir}/public-url-override.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string config.common.public_urls.proxy=https://proxy.example.test/remote >"${public_url_override}"
assert_contains "${public_url_override}" 'base_path: /remote'
assert_contains "${public_url_override}" 'proxy: https://proxy.example.test/remote'

admin_contact_render="${tmp_dir}/admin-contact.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string config.api.admin_contact_email=admin@example.test >"${admin_contact_render}"
assert_contains "${admin_contact_render}" 'admin_contact_email: admin@example.test'

invalid_path_render="${tmp_dir}/invalid-path.txt"
if helm template audit "${chart_dir}" "${required_values[@]}" \
  --set-string ingress.proxyPath=/remote/ >"${invalid_path_render}" 2>&1; then
  echo "expected a trailing slash in ingress.proxyPath to fail rendering" >&2
  exit 1
fi
assert_contains "${invalid_path_render}" 'ingress.proxyPath must be a normalized absolute non-root path'

proxy_only_render="${tmp_dir}/proxy-only.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set api.enabled=false >"${proxy_only_render}"
if [[ "$(grep -c '^kind: Ingress$' "${proxy_only_render}")" -ne 1 ]]; then
  echo "expected one proxy-only Ingress object" >&2
  exit 1
fi
assert_contains "${proxy_only_render}" 'path: "/remote"'
if grep -Fq -- 'name: audit-prompt-gate-api' "${proxy_only_render}"; then
  echo "proxy-only rendering must not reference the disabled API service" >&2
  exit 1
fi

api_only_render="${tmp_dir}/api-only.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set proxy.enabled=false >"${api_only_render}"
if [[ "$(grep -c '^kind: Ingress$' "${api_only_render}")" -ne 1 ]]; then
  echo "expected one API-only Ingress object" >&2
  exit 1
fi
assert_contains "${api_only_render}" 'path: /'
if grep -Fq -- 'name: audit-prompt-gate-proxy' "${api_only_render}"; then
  echo "API-only rendering must not reference the disabled proxy service" >&2
  exit 1
fi

no_ingress_render="${tmp_dir}/no-ingress.txt"
if helm template audit "${chart_dir}" "${required_values[@]}" \
  --set ingress.enabled=false >"${no_ingress_render}" 2>&1; then
  echo "expected an explicit public proxy URL when Ingress is disabled" >&2
  exit 1
fi
assert_contains "${no_ingress_render}" 'config.common.public_urls.proxy is required when the chart-managed proxy Ingress is disabled'

external_route_render="${tmp_dir}/external-route.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set ingress.enabled=false \
  --set-string config.proxy.base_path=/gateway \
  --set-string config.common.public_urls.proxy=https://proxy.example.test/gateway >"${external_route_render}"
assert_contains "${external_route_render}" 'base_path: /gateway'
assert_contains "${external_route_render}" 'proxy: https://proxy.example.test/gateway'
if grep -Eq -- '^kind: Ingress$' "${external_route_render}"; then
  echo "Ingress-disabled rendering must not contain an Ingress" >&2
  exit 1
fi

external_root_render="${tmp_dir}/external-root.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set ingress.enabled=false \
  --set-string config.common.public_urls.proxy=https://proxy.example.test/ >"${external_root_render}"
assert_contains "${external_root_render}" 'base_path: ""'
assert_contains "${external_root_render}" 'proxy: https://proxy.example.test/'

external_render="${tmp_dir}/external-secret.yaml"
helm template audit "${chart_dir}" \
  --set-string configSecret.existingSecret=promptgate-runtime-config \
  --set ingress.enabled=false \
  --set-string configSecret.rolloutToken=rotation-2 >"${external_render}"
assert_contains "${external_render}" 'secretName: promptgate-runtime-config'
assert_contains "${external_render}" 'prompt-gate.io/config-rollout: "rotation-2"'
if grep -Fq -- '"promptgate.yaml": |' "${external_render}"; then
  echo "the chart must not render configuration content when an existing Secret is used" >&2
  exit 1
fi

echo "Helm runtime configuration and ingress render checks passed"
