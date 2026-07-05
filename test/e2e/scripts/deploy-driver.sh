#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

require_cmd kubectl envsubst

if [[ -z "${THALASSA_CLIENT_ID:-}" || -z "${THALASSA_CLIENT_SECRET:-}" ]]; then
  echo "ERROR: THALASSA_CLIENT_ID and THALASSA_CLIENT_SECRET must be set" >&2
  exit 1
fi

if [[ -z "${THALASSA_ORGANISATION_ID:-}" ]]; then
  echo "ERROR: THALASSA_ORGANISATION_ID must be set" >&2
  exit 1
fi

export_env_for_manifests

KUBECONFIG="${KUBECONFIG:-$(read_state kubeconfig_path || true)}"
if [[ -z "${KUBECONFIG}" || ! -f "${KUBECONFIG}" ]]; then
  echo "ERROR: kubeconfig not found; run bootstrap-cluster.sh first or set KUBECONFIG" >&2
  exit 1
fi
export KUBECONFIG

echo "Deploying CSI driver ${E2E_DRIVER_NAME} with image ${E2E_IMAGE}"

kubectl apply -f <(envsubst <"${E2E_MANIFESTS_DIR}/namespace.yaml")
envsubst <"${E2E_MANIFESTS_DIR}/rbac.yaml" | kubectl apply -f -
envsubst <"${E2E_MANIFESTS_DIR}/csidriver.yaml" | kubectl apply -f -

kubectl -n "${E2E_NAMESPACE}" create secret generic thalassa-cloud-credentials \
  --from-literal=client_id="${THALASSA_CLIENT_ID}" \
  --from-literal=client_secret="${THALASSA_CLIENT_SECRET}" \
  --dry-run=client -o yaml | kubectl apply -f -

envsubst <"${E2E_MANIFESTS_DIR}/controller.yaml" | kubectl apply -f -
envsubst <"${E2E_MANIFESTS_DIR}/node.yaml" | kubectl apply -f -

kubectl -n "${E2E_NAMESPACE}" rollout status deployment/thalassa-csi-controller --timeout=10m
kubectl -n "${E2E_NAMESPACE}" rollout status daemonset/thalassa-csi-node --timeout=10m

kubectl -n "${E2E_NAMESPACE}" get pods -o wide
echo "CSI driver deployed."
