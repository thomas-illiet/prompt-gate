#!/usr/bin/env bash

set -euo pipefail

go_binary="${GO:-go}"
package_list="${GO_PACKAGES:-./...}"
minimum="${COVERAGE_MIN:-70}"
profile="${COVERAGE_PROFILE:-}"
remove_profile=false

if [[ ! "${minimum}" =~ ^([0-9]+([.][0-9]+)?|[.][0-9]+)$ ]] ||
  ! awk -v value="${minimum}" 'BEGIN { exit !(value >= 0 && value <= 100) }'; then
  echo "COVERAGE_MIN must be a number between 0 and 100, got: ${minimum}" >&2
  exit 2
fi

if [[ -z "${profile}" ]]; then
  profile="$(mktemp "${TMPDIR:-/tmp}/promptgate-coverage.XXXXXX")"
  remove_profile=true
fi

cleanup() {
  if [[ "${remove_profile}" == true ]]; then
    rm -f "${profile}"
  fi
}
trap cleanup EXIT

# GO_PACKAGES is produced by `go list` in the Makefile. Package import paths
# cannot contain whitespace, so intentional word splitting is safe here.
# shellcheck disable=SC2086
"${go_binary}" test ${package_list} -covermode=atomic -coverprofile="${profile}"

total="$({ "${go_binary}" tool cover -func="${profile}" || exit $?; } |
  awk '$1 == "total:" { gsub(/%/, "", $3); print $3 }')"
if [[ ! "${total}" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "unable to read total statement coverage from ${profile}" >&2
  exit 2
fi

printf 'Go statement coverage: %s%% (minimum: %s%%)\n' "${total}" "${minimum}"
if ! awk -v actual="${total}" -v required="${minimum}" \
  'BEGIN { exit !(actual + 0 >= required + 0) }'; then
  echo "coverage ${total}% is below the required ${minimum}%" >&2
  exit 1
fi
