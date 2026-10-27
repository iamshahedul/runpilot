#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "Building RunPilot release binaries..."
make -C "${ROOT_DIR}" build

echo "Packaging complete in ${ROOT_DIR}/bin"
ls -lh "${ROOT_DIR}/bin"
