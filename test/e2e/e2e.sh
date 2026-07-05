#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

K8S_VERSION="${K8S_VERSION:-$(kubectl version --output json | jq -r '.serverVersion.gitVersion')}"
export K8S_VERSION
export KUBE_SSH_USER="${KUBE_SSH_USER:-ubuntu}"
export KUBE_SSH_KEY_PATH="${KUBE_SSH_KEY_PATH:-/fake/path/to/skip/ssh/tests}"

if [[ "${K8S_VERSION}" == "null" || -z "${K8S_VERSION}" ]]; then
  echo "ERROR: Unable to get Kubernetes version. Set KUBECONFIG first." >&2
  exit 1
fi

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${arch}" in
  x86_64|amd64) test_arch="amd64" ;;
  aarch64|arm64) test_arch="arm64" ;;
  *)
    echo "ERROR: unsupported architecture for e2e.test: ${arch}" >&2
    exit 1
    ;;
esac

archive="kubernetes-test-${os}-${test_arch}.tar.gz"
echo "Downloading e2e.test for Kubernetes ${K8S_VERSION} (${archive})"
curl --location "https://dl.k8s.io/${K8S_VERSION}/${archive}" |
  tar --strip-components=3 -zxf - "kubernetes/test/bin/e2e.test"

E2E_DRIVER_NAME="${E2E_DRIVER_NAME:-csi.k8s.e2e.thalassa.cloud}"
E2E_TEST_PROFILE="${E2E_TEST_PROFILE:-full}"
E2E_GINKGO_SKIP="${E2E_GINKGO_SKIP:-Ephemeral-volume}"
E2E_TEST_DRIVER="${E2E_TEST_DRIVER:-test-driver.yaml}"

case "${E2E_TEST_PROFILE}" in
  full)
    E2E_GINKGO_FOCUS="${E2E_GINKGO_FOCUS:-External.Storage}"
    E2E_GINKGO_TIMEOUT="${E2E_GINKGO_TIMEOUT:-6h}"
    ;;
  smoke)
    E2E_GINKGO_FOCUS="${E2E_GINKGO_FOCUS:-External.Storage.*${E2E_DRIVER_NAME}.*Testpattern: Dynamic PV \\(ext4\\)}"
    E2E_GINKGO_TIMEOUT="${E2E_GINKGO_TIMEOUT:-1h}"
    ;;
  *)
    echo "ERROR: unknown E2E_TEST_PROFILE: ${E2E_TEST_PROFILE} (expected full or smoke)" >&2
    exit 1
    ;;
esac

echo "Running external storage e2e tests (profile: ${E2E_TEST_PROFILE})"
echo "  test driver: ${E2E_TEST_DRIVER}"
echo "  focus: ${E2E_GINKGO_FOCUS}"
echo "  skip: ${E2E_GINKGO_SKIP}"
echo "  timeout: ${E2E_GINKGO_TIMEOUT}"

./e2e.test \
  -ginkgo.focus="${E2E_GINKGO_FOCUS}" \
  -ginkgo.skip="${E2E_GINKGO_SKIP}" \
  -ginkgo.timeout="${E2E_GINKGO_TIMEOUT}" \
  -storage.testdriver="${E2E_TEST_DRIVER}"
