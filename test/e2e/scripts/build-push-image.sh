#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# shellcheck source=common.sh
source "${SCRIPT_DIR}/common.sh"

require_cmd docker go

E2E_IMAGE_PLATFORM="${E2E_IMAGE_PLATFORM:-linux/amd64}"

echo "Building CSI image ${E2E_IMAGE} for platform ${E2E_IMAGE_PLATFORM}"

pushd "${REPO_ROOT}" >/dev/null
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o csi-thalassa ./cmd/thalassa-csi-plugin/
docker build --platform "${E2E_IMAGE_PLATFORM}" -t "${E2E_IMAGE}" .
rm -f csi-thalassa
popd >/dev/null

if [[ "${E2E_REGISTRY}" == "ghcr.io" && -n "${GITHUB_TOKEN:-}" ]]; then
  echo "Logging in to ghcr.io with GITHUB_TOKEN"
  echo "${GITHUB_TOKEN}" | docker login ghcr.io \
    --username "${GITHUB_ACTOR:-github-actions[bot]}" \
    --password-stdin
elif [[ -n "${E2E_REGISTRY_USERNAME:-}" && -n "${E2E_REGISTRY_PASSWORD:-}" ]]; then
  echo "Logging in to ${E2E_REGISTRY}"
  echo "${E2E_REGISTRY_PASSWORD}" | docker login "${E2E_REGISTRY}" \
    --username "${E2E_REGISTRY_USERNAME}" \
    --password-stdin
else
  echo "WARNING: no registry credentials configured; docker push may fail" >&2
fi

echo "Pushing ${E2E_IMAGE}"
docker push "${E2E_IMAGE}"

echo "Image pushed: ${E2E_IMAGE}"
