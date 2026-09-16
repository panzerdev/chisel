#!/usr/bin/env bash
set -euo pipefail

# Registry und Image-Konfiguration
IMAGE_NAME="harbor.panzer.zone/images/chisel"

# Parameterprüfung
if [ -z "${1:-}" ]; then
  echo "Fehler: Kein Tag angegeben!"
  echo "Verwendung: $0 <tag>"
  echo "Beispiel:   $0 v1.0.0"
  exit 1
fi

TAG="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$SCRIPT_DIR"

echo "=========================================="
echo " Baue Chisel Docker Image"
echo " Image:   ${IMAGE_NAME}"
echo " Tags:    ${TAG} und latest"
echo " Version: ${TAG}"
echo "=========================================="

# Image mit übergebenem Tag und latest bauen
docker build \
  --build-arg VERSION="${TAG}" \
  -t "${IMAGE_NAME}:${TAG}" \
  -t "${IMAGE_NAME}:latest" \
  "${ROOT_DIR}"

echo ""
echo "=========================================="
echo " Pushe Images in die Registry"
echo "=========================================="

echo "-> Pushe ${IMAGE_NAME}:${TAG} ..."
docker push "${IMAGE_NAME}:${TAG}"

echo "-> Pushe ${IMAGE_NAME}:latest ..."
docker push "${IMAGE_NAME}:latest"

echo ""
echo "Erfolgreich gebaut und gepusht:"
echo "  - ${IMAGE_NAME}:${TAG}"
echo "  - ${IMAGE_NAME}:latest"
