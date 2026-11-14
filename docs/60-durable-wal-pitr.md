# Durable WAL archive + PITR from MinIO

Status: **passed** on 2026-11-05.

## Archive

- bucket `s3://aip-wal` (MinIO), `wal-g wal-push` with lz4
- `/home/postgres/pgdata/archive-wal.sh` sources `/home/postgres/pgdata/walg.env` (data volume, survives container recreate) (0600, `set -a`) and calls wal-g; no credentials in the command
- PostgreSQL `archive_command` is set both in Patroni DCS and `ALTER SYSTEM`
  (the DCS-only patch previously did not apply; docs/58)
- observed: `last_archived_wal=...26`, `archived_count=41`; `failed_count=3` is historical (the first env file did not export variables; fixed with `set -a`)
- MinIO lifecycle: `wal_005/` **30 days**, `basebackups_005/` 30 days (docs/63)

## PITR restore from MinIO

1. probe table row `1=before-base`; base backup
2. row `2` at `21:43:56.68`; row `3` at `21:44:00.66`; WAL switch
3. restore target `21:43:58` with
   `restore_command='/usr/local/bin/wal-g wal-fetch %f %p'`,
   `recovery_target_timeline='1'`
4. Spilo image (has wal-g/glibc), trust HBA, SSL off, no preload libraries

## Verified

```
starting point-in-time recovery to 2026-11-05 21:43:58+00
archive recovery complete
database system is ready to accept connections
1|before-base
2|after-base
```

Row 3 is absent. The archive now survives a node/local-disk loss (MinIO bucket).

## Cleanup

- restore container/volume removed; base backup and wal-g copy removed
- temporary replication HBA line reverted and reloaded
- probe table dropped; archive continues to run (`failed_count=0`)

## Limits / risks

- S3 credentials are in a postgres-owned 0600 env file on the node, not a secret
  manager
- WAL and base retention are aligned at 30 days; a restore older than that is
  impossible
- no retention/lifecycle policy on `aip-wal` yet
- restore used the Spilo image, not a Patroni rejoin
- target time is manual; no PITR API/automation
- archive_command is not yet reconciled with Patroni's bootstrap config on a full
  node recreate


## Monitoring

`GET /metrics` exposes:

```
aip_wal_archive_failed_total 3
aip_wal_archive_archived_total 41
aip_wal_archive_last_success_age_seconds ...
```

Prometheus alerts (`deploy/observability/alerts.yml`):

- `AIPWALArchiveFailing` — `increase(failed_total[10m]) > 0` (critical)
- `AIPWALArchiveStale` — no successful archive for > 1h (warning)

The historical `failed_total=3` does not page; only an increase does.
