# Offsite WAL/base mirror

Status: **implemented and exercised** on 2026-11-11.

## Topology

- second MinIO `minio-offsite` on `aip_default` (port 9002, own volume)
- `mc mirror --overwrite --remove local/aip-wal offsite/aip-wal`
- heartbeat `backup_heartbeat(kind='offsite')`; cron line 03:47 daily

## Verified

```
offsite_objects=77 heartbeat=updated
aip_offsite_backup_last_success_age_seconds 203.183
/dr offsite_age_seconds 429.6
```

The object counts matched source/target at mirror time. Prometheus alert
`AIPOffsiteBackupStale` fires after 48h without a mirror.

## Limits

- same host, separate container/volume — not a geographic region
- full-bucket mirror; no incremental/S3 replication semantics
- `--remove` follows source retention; a source deletion propagates
- offsite restore drill passed (docs/68)
- credentials for the offsite alias are in the minio container config
