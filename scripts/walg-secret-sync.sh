#!/usr/bin/env bash
# Sync the WAL S3 credentials from Vault dev into the Patroni node env file.
# This is a PoC: the Vault token is on the node/host, not a production auth method.
set -euo pipefail
VAULT_CONTAINER=${VAULT_CONTAINER:-aip-vault}
VAULT_TOKEN=${VAULT_TOKEN:-root}
PG_CONTAINER=${PG_CONTAINER:-aip-patroni-patroni1-1}
PATH_IN_VAULT=${PATH_IN_VAULT:-secret/walg}

json=$(docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN="$VAULT_TOKEN" "$VAULT_CONTAINER" \
    vault kv get -format=json "$PATH_IN_VAULT")
envfile=$(printf '%s' "$json" | python3 -c '
import json,sys
data=json.load(sys.stdin)["data"]["data"]
for k in ["WALG_S3_PREFIX","AWS_ACCESS_KEY_ID","AWS_SECRET_ACCESS_KEY","AWS_ENDPOINT","AWS_REGION","AWS_S3_FORCE_PATH_STYLE"]:
    if k in data: print(f"{k}={data[k]}")
')
printf '%s\n' "$envfile" | docker exec -i -u root "$PG_CONTAINER" sh -c \
    'cat > /home/postgres/walg.env.new && chown postgres:postgres /home/postgres/walg.env.new && chmod 600 /home/postgres/walg.env.new && mv /home/postgres/walg.env.new /home/postgres/walg.env'
echo "synced $(printf '%s\n' "$envfile" | wc -l) keys from $PATH_IN_VAULT"
