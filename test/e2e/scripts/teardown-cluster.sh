#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

require_cmd tcloud

if [[ ! -f "${E2E_STATE_FILE}" ]]; then
  echo "No e2e state file found at ${E2E_STATE_FILE}; nothing to tear down."
  exit 0
fi

THALASSA_require_auth

mapfile -t TCLOUD_ARGS < <(THALASSA_auth_args)

reload_tcloud_args() {
  mapfile -t TCLOUD_ARGS < <(THALASSA_auth_args)
}

CLUSTER_ID="$(read_state cluster_id || true)"
SUBNET_ID="$(read_state subnet_id || true)"
VPC_ID="$(read_state vpc_id || true)"
CLUSTER_NAME="$(read_state cluster_name || true)"
NAT_GATEWAY_ID="$(read_state nat_gateway_id || true)"
NAT_GATEWAY_NAME="$(read_state nat_gateway_name || true)"
SUBNET_NAME="$(read_state subnet_name || true)"
VPC_NAME="$(read_state vpc_name || true)"

if [[ -n "${CLUSTER_ID}" || -n "${CLUSTER_NAME}" ]]; then
  cluster_target="${CLUSTER_ID:-${CLUSTER_NAME}}"
  node_pool_name="$(read_state node_pool_name || true)"
  node_pool_name="${node_pool_name:-worker}"

  mapfile -t NODEPOOLS < <(
    tcloud "${TCLOUD_ARGS[@]}" kubernetes nodepools list --cluster "${cluster_target}" --no-header 2>/dev/null \
      | awk 'NF {print $1}' || true
  )
  if [[ ${#NODEPOOLS[@]} -eq 0 ]]; then
    NODEPOOLS=("${node_pool_name}")
  fi

  for nodepool in "${NODEPOOLS[@]}"; do
    delete_nodepool() {
      reload_tcloud_args
      tcloud "${TCLOUD_ARGS[@]}" kubernetes nodepools delete \
        --cluster "${cluster_target}" \
        --nodepool "${nodepool}" \
        --force --wait
    }
    echo "Deleting node pool ${nodepool} from cluster ${cluster_target}"
    run_with_retry "delete node pool ${nodepool}" delete_nodepool || true
  done

  delete_cluster() {
    reload_tcloud_args
    tcloud "${TCLOUD_ARGS[@]}" kubernetes delete "${cluster_target}" --force --wait
  }
  echo "Deleting Kubernetes cluster ${cluster_target}"
  run_with_retry "delete kubernetes cluster" delete_cluster || true
fi

if [[ -n "${NAT_GATEWAY_ID}" || -n "${NAT_GATEWAY_NAME}" ]]; then
  target="${NAT_GATEWAY_ID:-${NAT_GATEWAY_NAME}}"
  echo "Deleting NAT gateway ${target}"
  tcloud "${TCLOUD_ARGS[@]}" networking natgateways delete "${target}" --force --wait || true
fi

if [[ -n "${SUBNET_ID}" || -n "${SUBNET_NAME}" ]]; then
  target="${SUBNET_ID:-${SUBNET_NAME}}"
  echo "Deleting subnet ${target}"
  tcloud "${TCLOUD_ARGS[@]}" networking subnets delete "${target}" --force --wait 2>/dev/null || \
    tcloud "${TCLOUD_ARGS[@]}" networking subnets delete "${target}" --force 2>/dev/null || true
fi

if [[ -n "${VPC_ID}" || -n "${VPC_NAME}" ]]; then
  target="${VPC_ID:-${VPC_NAME}}"
  echo "Deleting VPC ${target}"
  tcloud "${TCLOUD_ARGS[@]}" networking vpcs delete "${target}" --force --wait 2>/dev/null || \
    tcloud "${TCLOUD_ARGS[@]}" networking vpcs delete "${target}" --force 2>/dev/null || true
fi

rm -f "${E2E_STATE_FILE}" "${E2E_ROOT}/kubeconfig"
echo "Teardown complete."
