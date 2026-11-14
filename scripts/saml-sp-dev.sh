#!/usr/bin/env bash
# Generate a persistent dev SAML SP key/cert into .cache/saml-sp (gitignored).
set -euo pipefail
DIR="$(cd "$(dirname "$0")/.." && pwd)/.cache/saml-sp"
mkdir -p "$DIR"
if [ -f "$DIR/key.pem" ] && [ -f "$DIR/cert.pem" ]; then
  echo "SP key/cert already exist in $DIR"
  exit 0
fi
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$DIR/key.pem" -out "$DIR/cert.pem" \
  -days 365 -subj /CN=aip-saml-sp >/dev/null 2>&1
echo "wrote $DIR/key.pem and $DIR/cert.pem"
echo "set AIP_SAML_KEY_FILE=/saml-sp/key.pem and AIP_SAML_CERT_FILE=/saml-sp/cert.pem"
echo "register cert.pem as the Keycloak client signing certificate (saml.signing.certificate)"
