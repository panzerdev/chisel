#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="${SCRIPT_DIR}/build"

# Version aus erstem Parameter oder Git-Tag ermitteln
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo "dev")}"

echo "=========================================="
echo " Baue Chisel Binaries für alle Plattformen"
echo " Version:    ${VERSION}"
echo " Zielordner: ${OUTPUT_DIR}"
echo "=========================================="

mkdir -p "${OUTPUT_DIR}"

# Baue alle Plattform-Binaries im Docker-Container und exportiere sie nach build/
docker build \
  -f "${SCRIPT_DIR}/Dockerfile.build" \
  --build-arg VERSION="${VERSION}" \
  --output "${OUTPUT_DIR}" \
  "${SCRIPT_DIR}"

echo ""
echo "=========================================="
echo " Fertig! Generierte Dateien in ${OUTPUT_DIR}:"
echo "=========================================="
ls -lh "${OUTPUT_DIR}"
