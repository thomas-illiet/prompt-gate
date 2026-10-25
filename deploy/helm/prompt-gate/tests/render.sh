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

helm lint "${chart_dir}" --strict "${required_values[@]}"

default_render="${tmp_dir}/default.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" >"${default_render}"
assert_contains "${default_render}" 'ingressClassName: "haproxy"'
assert_contains "${default_render}" 'haproxy.org/path-rewrite: ^/remote(/|$)(.*) /\2'
assert_contains "${default_render}" 'path: "/remote"'
assert_contains "${default_render}" 'pathType: Prefix'
assert_contains "${default_render}" 'kind: Secret'
assert_contains "${default_render}" '"promptgate.yaml": |'
assert_contains "${default_render}" 'database:'
assert_contains "${default_render}" 'url: postgres://user:pass@db/promptgate'
assert_contains "${default_render}" '/etc/prompt-gate/promptgate.yaml'
assert_contains "${default_render}" 'defaultMode: 0440'
if grep -Eq -- '^kind: ConfigMap$|envFrom:|PROMPTGATE_' "${default_render}"; then
  echo "runtime configuration must be mounted as YAML without PROMPTGATE_* environment variables" >&2
  exit 1
fi
if [[ "$(grep -c '^kind: Ingress$' "${default_render}")" -ne 2 ]]; then
  echo "expected the chart to render separate API and proxy Ingress objects" >&2
  exit 1
fi

no_strip_render="${tmp_dir}/no-strip.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --set ingress.proxy.stripPath=false >"${no_strip_render}"
if grep -Fq -- 'haproxy.org/path-rewrite:' "${no_strip_render}"; then
  echo "path rewrite must not be generated when ingress.proxy.stripPath=false" >&2
  exit 1
fi

override_render="${tmp_dir}/override.yaml"
helm template audit "${chart_dir}" "${required_values[@]}" \
  --values "${chart_dir}/tests/values-path-rewrite-override.yaml" >"${override_render}"
assert_contains "${override_render}" 'path: "/gateway"'
assert_contains "${override_render}" 'haproxy.org/path-rewrite: ^/custom(/|$)(.*) /backend/\2'

external_render="${tmp_dir}/external-secret.yaml"
helm template audit "${chart_dir}" \
  --set-string configSecret.existingSecret=promptgate-runtime-config \
  --set-string configSecret.rolloutToken=rotation-2 >"${external_render}"
assert_contains "${external_render}" 'secretName: promptgate-runtime-config'
assert_contains "${external_render}" 'prompt-gate.io/config-rollout: "rotation-2"'
if grep -Fq -- '"promptgate.yaml": |' "${external_render}"; then
  echo "the chart must not render configuration content when an existing Secret is used" >&2
  exit 1
fi

echo "Helm runtime configuration and ingress render checks passed"
