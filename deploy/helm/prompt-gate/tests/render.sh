#!/usr/bin/env bash

set -euo pipefail

chart_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq -- "${expected}" "${file}"; then
    echo "expected ${file} to contain: ${expected}" >&2
    exit 1
  fi
}

helm lint "${chart_dir}" --strict

default_render="${tmp_dir}/default.yaml"
helm template audit "${chart_dir}" >"${default_render}"
assert_contains "${default_render}" 'ingressClassName: "haproxy"'
assert_contains "${default_render}" 'haproxy.org/path-rewrite: ^/remote(/|$)(.*) /\2'
assert_contains "${default_render}" 'path: "/remote"'
assert_contains "${default_render}" 'pathType: Prefix'
if [[ "$(grep -c '^kind: Ingress$' "${default_render}")" -ne 2 ]]; then
  echo "expected the chart to render separate API and proxy Ingress objects" >&2
  exit 1
fi

no_strip_render="${tmp_dir}/no-strip.yaml"
helm template audit "${chart_dir}" \
  --set ingress.proxy.stripPath=false >"${no_strip_render}"
if grep -Fq -- 'haproxy.org/path-rewrite:' "${no_strip_render}"; then
  echo "path rewrite must not be generated when ingress.proxy.stripPath=false" >&2
  exit 1
fi

override_render="${tmp_dir}/override.yaml"
helm template audit "${chart_dir}" \
  --values "${chart_dir}/tests/values-path-rewrite-override.yaml" >"${override_render}"
assert_contains "${override_render}" 'path: "/gateway"'
assert_contains "${override_render}" 'haproxy.org/path-rewrite: ^/custom(/|$)(.*) /backend/\2'

echo "Helm ingress render checks passed"
