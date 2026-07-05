#!/usr/bin/env bash
set -euo pipefail

E2E_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
E2E_STATE_FILE="${E2E_STATE_FILE:-${E2E_ROOT}/.e2e-state}"
E2E_MANIFESTS_DIR="${E2E_ROOT}/manifests"

E2E_RUN_ID="${E2E_RUN_ID:-${GITHUB_RUN_ID:-local-$(date +%s)}}"
E2E_REGION="${E2E_REGION:-nl-01}"
E2E_AVAILABILITY_ZONE="${E2E_AVAILABILITY_ZONE:-nl-01a}"
E2E_MACHINE_TYPE="${E2E_MACHINE_TYPE:-pgp-medium}"
E2E_NODE_COUNT="${E2E_NODE_COUNT:-2}"
E2E_CLUSTER_VERSION="${E2E_CLUSTER_VERSION:-}"
E2E_POD_SECURITY_STANDARDS="${E2E_POD_SECURITY_STANDARDS:-privileged}"

E2E_REGISTRY="${E2E_REGISTRY:-registry.thalassacloud.nl}"
E2E_IMAGE_REPOSITORY="${E2E_IMAGE_REPOSITORY:-csi-thalassa-dev}"
_e2e_ref="${GITHUB_SHA:-}"
if [[ -z "${_e2e_ref}" ]]; then
  _e2e_ref="$(git -C "${E2E_ROOT}/../.." rev-parse HEAD 2>/dev/null || echo local)"
fi
E2E_IMAGE_TAG="${E2E_IMAGE_TAG:-e2e-${_e2e_ref}}"
E2E_IMAGE="${E2E_IMAGE:-${E2E_REGISTRY}/${E2E_IMAGE_REPOSITORY}:${E2E_IMAGE_TAG}}"

E2E_DRIVER_NAME="${E2E_DRIVER_NAME:-csi.k8s.e2e.thalassa.cloud}"
E2E_NAMESPACE="${E2E_NAMESPACE:-thalassa-system}"

THALASSA_API_URL="${THALASSA_API_URL:-https://api.thalassa.cloud}"
THALASSA_ORGANISATION_ID="${THALASSA_ORGANISATION_ID:-}"
THALASSA_PROJECT_ID="${THALASSA_PROJECT_ID:-}"

E2E_RESOURCE_PREFIX="${E2E_RESOURCE_PREFIX:-csi-e2e-${E2E_RUN_ID}}"
E2E_RETRY_ATTEMPTS="${E2E_RETRY_ATTEMPTS:-5}"
E2E_RETRY_DELAY_SECONDS="${E2E_RETRY_DELAY_SECONDS:-15}"

require_cmd() {
  for cmd in "$@"; do
    if ! command -v "${cmd}" >/dev/null 2>&1; then
      echo "ERROR: required command not found: ${cmd}" >&2
      exit 1
    fi
  done
}

THALASSA_auth_args() {
  local args=()
  if [[ -n "${THALASSA_CONTEXT:-}" ]]; then
    args+=(--context "${THALASSA_CONTEXT}")
  fi
  if [[ -n "${THALASSA_ORGANISATION:-}" ]]; then
    args+=(--organisation "${THALASSA_ORGANISATION_ID}")
  fi
  if [[ -n "${THALASSA_ACCESS_TOKEN:-}" ]]; then
    args+=(--access-token "${THALASSA_ACCESS_TOKEN}")
  fi
  if [[ -n "${THALASSA_TOKEN:-}" ]]; then
    args+=(--token "${THALASSA_TOKEN}")
  fi
  if [[ -n "${THALASSA_CLIENT_ID:-}" ]]; then
    args+=(--client-id "${THALASSA_CLIENT_ID}")
  fi
  if [[ -n "${THALASSA_CLIENT_SECRET:-}" ]]; then
    args+=(--client-secret "${THALASSA_CLIENT_SECRET}")
  fi
  if [[ -n "${THALASSA_API_URL:-}" ]]; then
    args+=(--api "${THALASSA_API_URL}")
  fi
  printf '%s\n' "${args[@]}"
}

THALASSA_has_auth() {
  [[ -n "${THALASSA_ACCESS_TOKEN:-}" ]] && return 0
  [[ -n "${THALASSA_TOKEN:-}" ]] && return 0
  [[ -n "${THALASSA_CLIENT_ID:-}" && -n "${THALASSA_CLIENT_SECRET:-}" ]] && return 0
  return 1
}

resolve_THALASSA_access_token() {
  if [[ -n "${THALASSA_ACCESS_TOKEN:-}" ]]; then
    return 0
  fi

  local subject_token="${THALASSA_ID_TOKEN:-${OIDC_TOKEN:-}}"
  if [[ -z "${subject_token}" && -n "${ACTIONS_ID_TOKEN_REQUEST_URL:-}" && -n "${ACTIONS_ID_TOKEN_REQUEST_TOKEN:-}" ]]; then
    require_cmd curl jq
    local audience="${THALASSA_OIDC_AUDIENCE:-thalassa-cloud}"
    subject_token="$(curl -sSf -H "Authorization: bearer ${ACTIONS_ID_TOKEN_REQUEST_TOKEN}" \
      "${ACTIONS_ID_TOKEN_REQUEST_URL}&audience=${audience}" | jq -r .value)"
    if [[ -z "${subject_token}" || "${subject_token}" == "null" ]]; then
      echo "ERROR: failed to obtain GitHub Actions OIDC token" >&2
      exit 1
    fi
  fi

  if [[ -z "${subject_token}" ]]; then
    return 0
  fi

  if [[ -z "${THALASSA_SERVICE_ACCOUNT_ID:-}" ]]; then
    echo "ERROR: THALASSA_SERVICE_ACCOUNT_ID is required for OIDC token exchange" >&2
    exit 1
  fi

  local org_id="${THALASSA_ORGANISATION_ID:-}"
  if [[ -z "${org_id}" ]]; then
    echo "ERROR: organisation ID is required for OIDC token exchange (set THALASSA_ORGANISATION_ID)" >&2
    exit 1
  fi

  require_cmd tcloud
  THALASSA_ACCESS_TOKEN="$(tcloud oidc token-exchange \
    --subject-token "${subject_token}" \
    --service-account-id "${THALASSA_SERVICE_ACCOUNT_ID}" \
    --organisation-id "${org_id}")"
  export THALASSA_ACCESS_TOKEN
}

THALASSA_require_auth() {
  resolve_THALASSA_access_token
  if ! THALASSA_has_auth; then
    echo "ERROR: configure THALASSA_ACCESS_TOKEN, THALASSA_TOKEN, OIDC token exchange (THALASSA_ID_TOKEN or GitHub Actions OIDC with THALASSA_SERVICE_ACCOUNT_ID), or THALASSA_CLIENT_ID/THALASSA_CLIENT_SECRET" >&2
    exit 1
  fi
}

refresh_THALASSA_auth() {
  unset THALASSA_ACCESS_TOKEN
  resolve_THALASSA_access_token
}

run_with_retry() {
  local description="$1"
  local fn="$2"
  local attempt=1
  local exit_code=0

  while (( attempt <= E2E_RETRY_ATTEMPTS )); do
    if (( attempt > 1 )); then
      echo "Retrying ${description} (attempt ${attempt}/${E2E_RETRY_ATTEMPTS})..." >&2
      refresh_THALASSA_auth
    fi

    set +e
    "${fn}"
    exit_code=$?
    set -e

    if [[ ${exit_code} -eq 0 ]]; then
      return 0
    fi

    if (( attempt >= E2E_RETRY_ATTEMPTS )); then
      echo "ERROR: ${description} failed after ${E2E_RETRY_ATTEMPTS} attempts" >&2
      return "${exit_code}"
    fi

    echo "WARNING: ${description} failed; waiting ${E2E_RETRY_DELAY_SECONDS}s before retry..." >&2
    sleep "${E2E_RETRY_DELAY_SECONDS}"
    attempt=$((attempt + 1))
  done
}

write_state() {
  local key="$1"
  local value="$2"
  mkdir -p "$(dirname "${E2E_STATE_FILE}")"
  touch "${E2E_STATE_FILE}"
  if grep -q "^${key}=" "${E2E_STATE_FILE}" 2>/dev/null; then
    sed -i.bak "s|^${key}=.*|${key}=${value}|" "${E2E_STATE_FILE}"
    rm -f "${E2E_STATE_FILE}.bak"
  else
    echo "${key}=${value}" >>"${E2E_STATE_FILE}"
  fi
}

read_state() {
  local key="$1"
  if [[ ! -f "${E2E_STATE_FILE}" ]]; then
    return 1
  fi
  grep "^${key}=" "${E2E_STATE_FILE}" | tail -n1 | cut -d= -f2-
}

export_env_for_manifests() {
  export E2E_IMAGE
  export E2E_DRIVER_NAME
  export E2E_NAMESPACE
  export E2E_REGION
  export THALASSA_API_URL
  export THALASSA_ORGANISATION_ID
  export THALASSA_PROJECT_ID="${THALASSA_PROJECT_ID:-}"
  export E2E_VPC_ID="${E2E_VPC_ID:-$(read_state vpc_id || true)}"
  export E2E_CLUSTER_ID="${E2E_CLUSTER_ID:-$(read_state cluster_id || true)}"
}
