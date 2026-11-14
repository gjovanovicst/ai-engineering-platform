#!/usr/bin/env bash
# Rotate the dev SAML SP key/cert, restart the API, re-register with Keycloak and
# optionally run the browser drill. This is not zero-downtime.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$ROOT/.cache/saml-sp"
STAMP=$(date +%Y%m%d-%H%M%S)
mkdir -p "$DIR"
if [ -f "$DIR/key.pem" ] && [ -f "$DIR/cert.pem" ]; then
  cp "$DIR/key.pem" "$DIR/key.pem.$STAMP.bak"
  cp "$DIR/cert.pem" "$DIR/cert.pem.$STAMP.bak"
fi
openssl req -x509 -newkey rsa:2048 -nodes -keyout "$DIR/key.pem.new" -out "$DIR/cert.pem.new" \
  -days 365 -subj /CN=aip-saml-sp >/dev/null 2>&1
mv "$DIR/key.pem.new" "$DIR/key.pem"
mv "$DIR/cert.pem.new" "$DIR/cert.pem"
echo "rotated SP key/cert (backup suffix $STAMP)"
docker compose up -d --force-recreate api >/dev/null
for _ in $(seq 1 30); do
  [ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:18080/healthz)" = "200" ] && break
  sleep 1
done
bash "$ROOT/scripts/keycloak-saml-client.sh" "$DIR/cert.pem" >/dev/null
echo "registered new certificate with Keycloak"
if [ "${AIP_RUN_DRILL:-0}" = "1" ]; then
  HOME="${HOME:-/tmp}" NODE_PATH="${NODE_PATH:-}" node "$ROOT/scripts/saml-drill.js"
fi
