#!/bin/sh
# Verify a signed API image before deployment (keyless cosign).
set -eu

IMAGE=${1:?usage: verify-image.sh <image@digest>}
IDENTITY=${AIP_COSIGN_IDENTITY_REGEXP:?AIP_COSIGN_IDENTITY_REGEXP is required (e.g. ^https://github.com/org/repo/)}
ISSUER=${AIP_COSIGN_OIDC_ISSUER:-https://token.actions.githubusercontent.com}

if ! command -v cosign >/dev/null 2>&1; then
    echo "[verify-image] cosign is not installed" >&2
    exit 1
fi

echo "[verify-image] verifying $IMAGE"
cosign verify \
    --certificate-identity-regexp "$IDENTITY" \
    --certificate-oidc-issuer "$ISSUER" \
    "$IMAGE"
echo "[verify-image] OK"
