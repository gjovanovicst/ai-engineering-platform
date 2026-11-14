#!/usr/bin/env bash
# Sync the WAL S3 credentials using a Vault AppRole and a short-lived token.
set -euo pipefail
VAULT_CONTAINER=${VAULT_CONTAINER:-aip-vault}
PG_CONTAINER=${PG_CONTAINER:-aip-patroni-patroni1-1}
PATH_IN_VAULT=${PATH_IN_VAULT:-secret/walg}
CREDS=${CREDS:-.cache/vault-approle}

if [ -z "${ROLE_ID:-}" ] || [ -z "${SECRET_ID:-}" ]; then
    # shellcheck disable=SC1090
    . "$CREDS"
fi

json=$(docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e ROLE_ID="$ROLE_ID" -e SECRET_ID="$SECRET_ID" "$VAULT_CONTAINER" \
    sh -c 'TOKEN=$(vault write -field=token auth/approle/login role_id="$ROLE_ID" secret_id="$SECRET_ID") && VAULT_TOKEN="$TOKEN" vault kv get -format=json "$0"' "$PATH_IN_VAULT")
envfile=$(printf '%s' "$json" | python3 -c '
import json,sys
data=json.load(sys.stdin)["data"]["data"]
for k in ["WALG_S3_PREFIX","AWS_ACCESS_KEY_ID","AWS_SECRET_ACCESS_KEY","AWS_ENDPOINT","AWS_REGION","AWS_S3_FORCE_PATH_STYLE"]:
    if k in data: print(f"{k}={data[k]}")
')
printf '%s\n' "$envfile" | docker exec -i -u root "$PG_CONTAINER" sh -c \
    'cat > /home/postgres/walg.env.new && chown postgres:postgres /home/postgres/walg.env.new && chmod 600 /home/postgres/walg.env.new && mv /home/postgres/walg.env.new /home/postgres/walg.env'
echo "synced $(printf '%s\n' "$envfile" | wc -l) keys via AppRole"
