#!/usr/bin/env bash

set -euo pipefail

example_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${example_dir}/../../.." && pwd)"
tmp_dir="$(mktemp -d)"
server_pid=""
container_name=""
cleanup() {
  if [[ -n "${server_pid}" ]]; then
    kill "${server_pid}" 2>/dev/null || true
    wait "${server_pid}" 2>/dev/null || true
  fi
  if [[ -n "${container_name}" ]]; then
    docker rm -f "${container_name}" >/dev/null 2>&1 || true
  fi
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

embedded_policy="${tmp_dir}/policy.rego"
normalized_policy="${tmp_dir}/normalized-policy.rego"
embedded_authz="${tmp_dir}/authz.rego"
normalized_authz="${tmp_dir}/normalized-authz.rego"
awk '
  /^  policy\.rego: \|$/ { in_policy = 1; next }
  in_policy && /^  authz\.rego: \|$/ { exit }
  in_policy { sub(/^    /, ""); print }
' "${example_dir}/kubernetes.yaml" >"${embedded_policy}"
awk '
  /^  authz\.rego: \|$/ { in_policy = 1; next }
  in_policy && /^---$/ { exit }
  in_policy { sub(/^    /, ""); print }
' "${example_dir}/kubernetes.yaml" >"${embedded_authz}"

# YAML examples conventionally use spaces while `opa fmt` uses tabs. Normalize
# only leading Rego indentation so every policy token still has to match.
sed 's/^\t/  /' "${example_dir}/policy.rego" >"${normalized_policy}"
sed 's/^\t/  /' "${example_dir}/authz.rego" >"${normalized_authz}"
diff -u "${normalized_policy}" "${embedded_policy}"
diff -u "${normalized_authz}" "${embedded_authz}"

if command -v opa >/dev/null 2>&1; then
  opa test \
    "${example_dir}/policy.rego" "${example_dir}/policy_test.rego" \
    "${example_dir}/authz.rego" "${example_dir}/authz_test.rego"
elif command -v docker >/dev/null 2>&1; then
  docker run --rm \
    -v "${repo_root}:/workspace:ro" \
    -w /workspace \
    openpolicyagent/opa:1.21.0-static \
    test \
      deploy/examples/opa/policy.rego deploy/examples/opa/policy_test.rego \
      deploy/examples/opa/authz.rego deploy/examples/opa/authz_test.rego
else
  echo "OPA validation requires either opa or docker" >&2
  exit 1
fi

policy_dir="${tmp_dir}/policies"
mkdir -p "${policy_dir}"
cp "${example_dir}/policy.rego" "${policy_dir}/policy.rego"
cp "${example_dir}/authz.rego" "${policy_dir}/authz.rego"

if command -v opa >/dev/null 2>&1; then
  opa_port="${PROMPTGATE_OPA_TEST_PORT:-18181}"
  opa run --server --authorization=basic --addr="127.0.0.1:${opa_port}" "${policy_dir}" >"${tmp_dir}/opa.log" 2>&1 &
  server_pid="$!"
  opa_url="http://127.0.0.1:${opa_port}"
else
  container_name="promptgate-opa-test-$$-${RANDOM}"
  docker run --detach --rm \
    --name "${container_name}" \
    --publish 127.0.0.1::8181 \
    --volume "${policy_dir}:/policies:ro" \
    openpolicyagent/opa:1.21.0-static \
    run --server --authorization=basic --addr=0.0.0.0:8181 /policies >/dev/null
  published="$(docker port "${container_name}" 8181/tcp)"
  opa_url="http://${published}"
fi

for _ in $(seq 1 50); do
  if curl --fail --silent --show-error "${opa_url}/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
curl --fail --silent --show-error "${opa_url}/health" >/dev/null

decision_payload='{"input":{"identity":{"id":"admin-id","type":"user","role":"admin","groups":[]},"request":{"client_ip":"203.0.113.4","method":"POST","path":"/openai-main/v1/responses","host":"proxy.example.com","provider":"openai-main","provider_type":"openai","provider_path":"/v1/responses"}}}'
decision_response="$(curl --fail --silent --show-error \
  -H 'Content-Type: application/json' \
  -d "${decision_payload}" \
  "${opa_url}/v1/data/promptgate/proxy/decision")"
if [[ "${decision_response}" != *'"allow":true'* ]]; then
  echo "expected allowed decision response, got: ${decision_response}" >&2
  exit 1
fi

policy_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -X PUT -H 'Content-Type: text/plain' --data 'package promptgate.proxy' \
  "${opa_url}/v1/policies/evil")"
if [[ "${policy_status}" != "401" ]]; then
  echo "expected policy mutation to be rejected with 401, got ${policy_status}" >&2
  exit 1
fi

data_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  -X PUT -H 'Content-Type: application/json' --data '{"owned":true}' \
  "${opa_url}/v1/data/promptgate")"
if [[ "${data_status}" != "401" ]]; then
  echo "expected data mutation to be rejected with 401, got ${data_status}" >&2
  exit 1
fi
