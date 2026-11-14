# Vault AppRole for WAL credentials

Status: **PoC drill** on 2026-11-05. Better than the root-token PoC in docs/61,
but still not production orchestration.

## Setup (Vault dev)

- policy `walg-read`: read on `secret/data/walg`
- AppRole `walg`: policy `walg-read`, `token_ttl=60s`, `secret_id_ttl=0`
- `role_id`/`secret_id` were kept outside git (`.cache/vault-approle`, removed)

## Sync

`scripts/walg-secret-sync-approle.sh`:

1. logs in with AppRole inside the Vault container
2. gets a 60s token
3. reads `secret/walg`
4. atomically rewrites `/home/postgres/walg.env` (0600)

## Verified

```
synced 6 keys via AppRole
last_archived_wal=...2D, archived_count=48, failed_count=6 (historical)
```

The archive continued after the sync.

## Limits

- Vault **dev mode**; no TLS, no real storage
- `secret_id` is on the host; no response wrapping
- sync is manual, not a sidecar/timer
- no AppRole audit trail of who triggered the sync
- Vault was removed after the drill; the synced env file remains valid
