# Vault-synced WAL credentials (PoC)

Status: **PoC drill** on 2026-11-05. Not production auth: Vault dev mode and a
root token are used. See docs/62 for the AppRole follow-up.

## Flow

1. Vault dev (`VAULT_DEV_ROOT_TOKEN_ID=root`) stores
   `secret/walg` (S3 prefix, endpoint, access key/secret).
2. `scripts/walg-secret-sync.sh` reads the KV entry and atomically rewrites
   `/home/postgres/walg.env` (0600, postgres) on the Patroni node.
3. `archive-wal.sh` sources that file and calls `wal-g`.

## Verified

```
synced 6 keys from secret/walg
```

- archive continued: `last_archived_wal=...2A`, archived 45
- rotation to a wrong secret: `last_failed_wal=...2B`, `failed_count=6`
- rotation back to the correct secret: `last_archived_wal=...2C`, archived 47

So the archive follows the managed secret, and a bad rotation is visible through
the failure counter/alerts.

## Limits

- Vault dev + root token; no AppRole/Kubernetes auth
- the sync is **manual** (`scripts/walg-secret-sync.sh`), not a sidecar/timer
- the token is on the host, not injected by an orchestrator
- no automatic rotation; no audit trail of the sync
- Vault was removed after the drill; the synced env file remains
