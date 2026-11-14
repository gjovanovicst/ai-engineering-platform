#!/usr/bin/env bash
# Manual recovery from a stale Patroni synchronous-mode key.
# Deletes /service/<scope>/sync ONLY when the leader key is absent, the old
# primary is confirmed down, and the operator passes explicit confirmations.
set -euo pipefail
SCOPE=${SCOPE:-aip-ha}
ETCD=${ETCD:?set ETCD to the etcd container name}
SURVIVOR=${SURVIVOR:?set SURVIVOR to the expected surviving member name}

if [ "${PRIMARY_DOWN:-}" != "yes" ]; then
    echo "refusing: set PRIMARY_DOWN=yes only after the old primary is down/fenced" >&2
    exit 1
fi
if [ "${CONFIRM:-}" != "$SCOPE" ]; then
    echo "refusing: set CONFIRM=$SCOPE to confirm the scope" >&2
    exit 1
fi

etcd() { docker exec -e ETCDCTL_API=2 "$ETCD" etcdctl "$@"; }

if etcd get "/service/$SCOPE/leader" >/dev/null 2>&1; then
    echo "refusing: a leader key still exists; do not delete the sync key" >&2
    exit 1
fi
sync_value=$(etcd get "/service/$SCOPE/sync" 2>/dev/null || true)
if [ -z "$sync_value" ]; then
    echo "no sync key; nothing to do"
    exit 0
fi
echo "stale_sync=$sync_value"
echo "survivor=$SURVIVOR"
if ! etcd get "/service/$SCOPE/members/$SURVIVOR" >/dev/null 2>&1; then
    echo "refusing: survivor member key not found; check the cluster" >&2
    exit 1
fi

etcd rm "/service/$SCOPE/sync"
echo "removed /service/$SCOPE/sync; watch the survivor promote"
