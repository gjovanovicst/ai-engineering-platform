# MinIO base backup + PITR

Status: **passed** on 2026-11-05.

## Backup

`wal-g backup-push` was run as `postgres` with `PGHOST=/var/run/postgresql`:

```
backup_name                   modified             wal_file_name            storage
base_00000001000000000000002F 2026-11-05T23:52:05Z 00000001000000000000002F default
```

Objects live under `basebackups_005/` in `s3://aip-wal`.

## Retention

MinIO lifecycle rules:

| Prefix | Expire |
|---|---|
| `wal_005/` | 30 days |
| `basebackups_005/` | 30 days |

They were aligned to 30/30: a 30-day base with 14-day WAL would leave the first
16 days unrestorable for PITR.

## PITR from MinIO

- `wal-g backup-fetch /data base_...2F` into a scratch volume
- restore target **23:52:26.5** (between marker rows 2 and 3)
- `restore_command = /usr/local/bin/wal-g wal-fetch %f %p`,
  `recovery_target_timeline=1`
- Spilo image, trust HBA, SSL off, no preload libraries

Verified:

```
archive recovery complete
database system is ready to accept connections
1|before-base
2|after-base
```

Row 3 is absent.

## Cleanup

- restore container/volume removed, probe table dropped
- archive continues (`last_archived_wal=...30`, 52 archived)
- Vault was already removed; the synced env file is used

## Gaps

- no automated base-backup schedule (manual `backup-push`)
- no alert when the newest base backup is too old
- no `wal-g delete retain` policy beyond MinIO lifecycle
- restore is standalone, not a Patroni node
