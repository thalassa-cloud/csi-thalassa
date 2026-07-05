#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

export E2E_RUN_ID="${E2E_RUN_ID:-${GITHUB_RUN_ID:-local-$(date +%s)}}"

cleanup() {
  local exit_code=$?
  if [[ "${E2E_SKIP_TEARDOWN:-false}" != "true" ]]; then
    echo "Tearing down e2e infrastructure..."
    "${SCRIPT_DIR}/teardown-cluster.sh" || true
  fi
  exit "${exit_code}"
}
trap cleanup EXIT

"${SCRIPT_DIR}/install-tcloud.sh"
"${SCRIPT_DIR}/bootstrap-cluster.sh"
"${SCRIPT_DIR}/build-push-image.sh"
"${SCRIPT_DIR}/deploy-driver.sh"
"${E2E_ROOT}/e2e.sh"
