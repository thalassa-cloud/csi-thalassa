#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

require_cmd tcloud curl awk grep jq

THALASSA_require_auth

mapfile -t TCLOUD_ARGS < <(THALASSA_auth_args)

VPC_NAME="${E2E_RESOURCE_PREFIX}-vpc"
SUBNET_NAME="${E2E_RESOURCE_PREFIX}-subnet"
NAT_GATEWAY_NAME="${E2E_RESOURCE_PREFIX}-nat"
CLUSTER_NAME="${E2E_RESOURCE_PREFIX}-cluster"
KUBECONFIG_PATH="${E2E_ROOT}/kubeconfig"
E2E_LABEL="csi-e2e/run-id=${E2E_RUN_ID}"
E2E_NODE_POOL_NAME="${E2E_NODE_POOL_NAME:-worker}"

echo "Bootstrapping Thalassa Cloud resources for e2e run ${E2E_RUN_ID}"

reload_tcloud_args() {
  mapfile -t TCLOUD_ARGS < <(THALASSA_auth_args)
}

create_vpc() {
  reload_tcloud_args
  tcloud "${TCLOUD_ARGS[@]}" networking vpcs create --name "${VPC_NAME}" \
    --region "${E2E_REGION}" \
    --cidrs 10.240.0.0/16 \
    --labels "${E2E_LABEL}" \
    --wait
}

echo "Creating VPC ${VPC_NAME} in region ${E2E_REGION}"
run_with_retry "VPC create" create_vpc

reload_tcloud_args
VPC_ID="$(tcloud "${TCLOUD_ARGS[@]}" networking vpcs list --selector "${E2E_LABEL}" --no-header | awk 'NR==1 {print $1}')"
if [[ -z "${VPC_ID}" ]]; then
  echo "ERROR: failed to resolve VPC identity after create" >&2
  exit 1
fi
echo "VPC identity: ${VPC_ID}"
write_state vpc_id "${VPC_ID}"
write_state vpc_name "${VPC_NAME}"

create_subnet() {
  reload_tcloud_args
  tcloud "${TCLOUD_ARGS[@]}" networking subnets create --name "${SUBNET_NAME}" \
    --vpc "${VPC_ID}" \
    --cidr 10.240.0.0/24 \
    --wait
}

echo "Creating subnet ${SUBNET_NAME}"
run_with_retry "subnet create" create_subnet

reload_tcloud_args
SUBNET_ID="$(tcloud "${TCLOUD_ARGS[@]}" networking subnets list --vpc "${VPC_ID}" --no-header | awk -v name="${SUBNET_NAME}" '$0 ~ name {print $1; exit}')"
if [[ -z "${SUBNET_ID}" ]]; then
  echo "ERROR: failed to resolve subnet identity after create" >&2
  exit 1
fi
echo "Subnet identity: ${SUBNET_ID}"
write_state subnet_id "${SUBNET_ID}"
write_state subnet_name "${SUBNET_NAME}"

create_nat_gateway() {
  reload_tcloud_args
  local body response nat_id
  body="$(jq -nc \
    --arg name "${NAT_GATEWAY_NAME}" \
    --arg subnet "${SUBNET_ID}" \
    --arg run_id "${E2E_RUN_ID}" \
    '{
      name: $name,
      description: "CSI e2e NAT gateway",
      subnetIdentity: $subnet,
      configureDefaultRoute: true,
      labels: {"csi-e2e/run-id": $run_id}
    }')"
  response="$(tcloud "${TCLOUD_ARGS[@]}" api raw -X POST -d "${body}" /v1/nat-gateways)"
  nat_id="$(echo "${response}" | jq -r .identity)"
  if [[ -z "${nat_id}" || "${nat_id}" == "null" ]]; then
    echo "${response}" >&2
    return 1
  fi
  NAT_GATEWAY_ID="${nat_id}"
}

echo "Creating NAT gateway ${NAT_GATEWAY_NAME}"
if ! run_with_retry "NAT gateway create" create_nat_gateway; then
  reload_tcloud_args
  NAT_GATEWAY_ID="$(tcloud "${TCLOUD_ARGS[@]}" networking natgateways list --selector "csi-e2e/run-id=${E2E_RUN_ID}" --no-header 2>/dev/null | awk 'NR==1 {print $1}')"
  if [[ -z "${NAT_GATEWAY_ID}" ]]; then
    echo "ERROR: failed to create NAT gateway" >&2
    exit 1
  fi
  echo "NAT gateway already exists: ${NAT_GATEWAY_ID}"
fi
echo "NAT gateway identity: ${NAT_GATEWAY_ID}"
write_state nat_gateway_id "${NAT_GATEWAY_ID}"
write_state nat_gateway_name "${NAT_GATEWAY_NAME}"

wait_for_nat_gateway_endpoint() {
  reload_tcloud_args
  NAT_ENDPOINT_IP="$(tcloud "${TCLOUD_ARGS[@]}" api raw "/v1/nat-gateways/${NAT_GATEWAY_ID}" | jq -r '.endpointIP // empty')"
  [[ -n "${NAT_ENDPOINT_IP}" ]]
}

echo "Waiting for NAT gateway ${NAT_GATEWAY_ID} endpoint"
if ! run_with_retry "NAT gateway endpoint" wait_for_nat_gateway_endpoint; then
  echo "ERROR: timed out waiting for NAT gateway ${NAT_GATEWAY_ID} endpoint" >&2
  exit 1
fi
echo "NAT gateway endpoint: ${NAT_ENDPOINT_IP}"

create_kubernetes_cluster() {
  reload_tcloud_args
  local -a args=(
    "${TCLOUD_ARGS[@]}"
    kubernetes create "${CLUSTER_NAME}"
    --subnet "${SUBNET_ID}"
    --machine-type "${E2E_MACHINE_TYPE}"
    --num-nodes "${E2E_NODE_COUNT}"
    --node-pool-name "${E2E_NODE_POOL_NAME}"
    --availability-zone "${E2E_AVAILABILITY_ZONE}"
    --pod-security-standards "${E2E_POD_SECURITY_STANDARDS}"
    --labels "${E2E_LABEL}"
    --wait
  )
  if [[ -n "${E2E_CLUSTER_VERSION}" ]]; then
    args+=(--cluster-version "${E2E_CLUSTER_VERSION}")
  fi
  tcloud "${args[@]}"
}

resolve_cluster_id() {
  reload_tcloud_args
  CLUSTER_ID="$(tcloud "${TCLOUD_ARGS[@]}" kubernetes list --vpc "${VPC_ID}" --no-header | awk -v name="${CLUSTER_NAME}" '$0 ~ name {print $1; exit}')"
  [[ -n "${CLUSTER_ID}" ]]
}

write_kubeconfig() {
  reload_tcloud_args
  tcloud "${TCLOUD_ARGS[@]}" kubernetes kubeconfig "${CLUSTER_ID}" >"${KUBECONFIG_PATH}"
}

wait_for_kubernetes_nodes() {
  export KUBECONFIG="${KUBECONFIG_PATH}"
  local ready_nodes
  ready_nodes="$(kubectl get nodes --no-headers 2>/dev/null | awk '$2 == "Ready" { count++ } END { print count + 0 }')"
  [[ "${ready_nodes}" -ge "${E2E_NODE_COUNT}" ]]
}

echo "Creating Kubernetes cluster ${CLUSTER_NAME}"
if ! run_with_retry "kubernetes cluster create" create_kubernetes_cluster; then
  echo "WARNING: kubernetes cluster create failed; checking for partially provisioned cluster..." >&2
fi

if ! run_with_retry "resolve kubernetes cluster identity" resolve_cluster_id; then
  echo "ERROR: failed to resolve cluster identity after create" >&2
  exit 1
fi
echo "Cluster identity: ${CLUSTER_ID}"
write_state cluster_id "${CLUSTER_ID}"
write_state cluster_name "${CLUSTER_NAME}"
write_state node_pool_name "${E2E_NODE_POOL_NAME}"
write_state run_id "${E2E_RUN_ID}"

echo "Writing kubeconfig to ${KUBECONFIG_PATH}"
run_with_retry "write kubeconfig" write_kubeconfig
write_state kubeconfig_path "${KUBECONFIG_PATH}"

echo "Waiting for ${E2E_NODE_COUNT} ready Kubernetes node(s)"
if ! run_with_retry "kubernetes nodes ready" wait_for_kubernetes_nodes; then
  echo "ERROR: timed out waiting for Kubernetes nodes to become ready" >&2
  exit 1
fi

export KUBECONFIG="${KUBECONFIG_PATH}"
require_cmd kubectl
kubectl get nodes

echo "Bootstrap complete."
echo "  VPC:         ${VPC_ID}"
echo "  Subnet:      ${SUBNET_ID}"
echo "  NAT gateway: ${NAT_GATEWAY_ID} (${NAT_ENDPOINT_IP})"
echo "  Cluster:     ${CLUSTER_ID}"
echo "  Kubeconfig:  ${KUBECONFIG_PATH}"
