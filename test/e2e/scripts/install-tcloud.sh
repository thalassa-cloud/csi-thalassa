#!/usr/bin/env bash
set -euo pipefail

THALASSA_VERSION="${THALASSA_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

arch="$(uname -m)"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"

case "${arch}" in
  x86_64|amd64) THALASSA_arch="amd64" ;;
  aarch64|arm64) THALASSA_arch="arm64" ;;
  *)
    echo "ERROR: unsupported architecture: ${arch}" >&2
    exit 1
    ;;
esac

case "${os}" in
  linux|darwin) ;;
  *)
    echo "ERROR: unsupported operating system: ${os}" >&2
    exit 1
    ;;
esac

if command -v tcloud >/dev/null 2>&1; then
  echo "tcloud already installed: $(tcloud version)"
  exit 0
fi

mkdir -p "${INSTALL_DIR}"

if [[ "${THALASSA_VERSION}" == "latest" ]]; then
  release_url="https://github.com/thalassa-cloud/cli/releases/latest/download/tcloud-${os}-${THALASSA_arch}"
else
  release_url="https://github.com/thalassa-cloud/cli/releases/download/${THALASSA_VERSION}/tcloud-${os}-${THALASSA_arch}"
fi

echo "Installing tcloud from ${release_url}"
curl -fsSL "${release_url}" -o "${INSTALL_DIR}/tcloud"
chmod +x "${INSTALL_DIR}/tcloud"

echo "Installed $( "${INSTALL_DIR}/tcloud" version )"
echo "Make sure ${INSTALL_DIR} is on your PATH"
