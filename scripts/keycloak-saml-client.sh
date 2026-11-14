#!/usr/bin/env bash
# Register the dev SP certificate and signing/encryption settings on the Keycloak
# client. Requires the Keycloak container and kcadm admin credentials.
set -euo pipefail
CONTAINER=${KEYCLOAK_CONTAINER:-aip-keycloak-1}
REALM=${KEYCLOAK_REALM:-aip}
CLIENT=${KEYCLOAK_SAML_CLIENT:-aip-saml}
CERT=${1:-.cache/saml-sp/cert.pem}
ENCRYPT=${AIP_SAML_ENCRYPT:-true}

[ -f "$CERT" ] || { echo "missing $CERT (run scripts/saml-sp-dev.sh)" >&2; exit 1; }

docker exec "$CONTAINER" /opt/keycloak/bin/kcadm.sh config credentials \
  --server http://localhost:8080 --realm master --user admin --password admin >/dev/null
CID=$(docker exec "$CONTAINER" /opt/keycloak/bin/kcadm.sh get clients -r "$REALM" \
  -q clientId="$CLIENT" --fields id --format csv --noquotes | tail -1)
[ -n "$CID" ] || { echo "client $CLIENT not found" >&2; exit 1; }
docker exec "$CONTAINER" /opt/keycloak/bin/kcadm.sh get clients/"$CID" -r "$REALM" > /tmp/kc-saml-client.json
CERT_PATH="$CERT" ENCRYPT="$ENCRYPT" python3 - <<'PY'
import json, os, pathlib
p = pathlib.Path("/tmp/kc-saml-client.json")
data = json.loads(p.read_text())
attrs = data.get("attributes") or {}
cert = pathlib.Path(os.environ["CERT_PATH"]).read_text()
attrs["saml.signing.certificate"] = cert
attrs["saml.client.signature"] = "true"
attrs["saml.encrypt"] = os.environ["ENCRYPT"]
if os.environ["ENCRYPT"] == "true":
    attrs["saml.encryption.certificate"] = cert
    attrs["saml.encryption.algorithm"] = "RSA-OAEP"
attrs["saml_single_logout_service_url_post"] = os.environ.get("AIP_API_URL", "http://localhost:18080") + "/saml/slo"
data["attributes"] = attrs
p.write_text(json.dumps(data))
print("updated client attributes")
PY
docker cp /tmp/kc-saml-client.json "$CONTAINER":/tmp/kc-saml-client.json
docker exec "$CONTAINER" /opt/keycloak/bin/kcadm.sh update clients/"$CID" -r "$REALM" -f /tmp/kc-saml-client.json >/dev/null
echo "Keycloak client $CLIENT updated"
