# Base backup schedule heartbeat

Status: **wired** on 2026-11-05; the cron entry is an install artifact, not active
on this WSL host.

## Script

`scripts/walg-base-backup.sh`:

- takes a file lock (one backup at a time)
- runs `wal-g backup-push` as postgres (local socket)
- reads the newest backup name
- upserts `backup_heartbeat(kind='base', last_success_at=now(), detail=name)`

Cron artifact: `deploy/backup/aip-base-backup.cron` (03:17 daily).

## Monitoring

`/metrics`:

```
aip_base_backup_last_success_age_seconds ...
```

Prometheus alert:

```
AIPBaseBackupStale   aip_base_backup_last_success_age_seconds > 172800   critical
```

(48 hours; for a daily backup this catches two missed runs.)

## Verified

```
base_backup=base_000000010000000000000036 heartbeat=updated
aip_base_backup_last_success_age_seconds 234.601
```

Metric load requires `GRANT SELECT ON backup_heartbeat TO aip_app`; the migration
was applied as `postgres`, so the default-privilege grant did not apply here.

## Gaps

- cron is not installed in this environment; no scheduler proof
- no failure heartbeat (only success updates the row; staleness covers failure)
- no offsite/region copy of the MinIO bucket
