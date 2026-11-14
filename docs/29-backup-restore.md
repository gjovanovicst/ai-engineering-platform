# Backup and restore

Status: **verified 2026-10-26** · owner: platform

## What exists

- `scripts/backup.sh` — `pg_dump --format=custom`, SHA-256 sidecar, retention
  (`RETENTION_DAYS`, default 14), never deletes the newest dump.
- `scripts/restore-check.sh` — restores a dump into a scratch database and runs
  sanity counts; used by the restore drill.
- `make backup` / `make restore-check` (require `AIP_DB_DSN` and host `pg_dump`).

## Verified drill (2026-10-26)

Environment: local docker compose, 12 orgs, 373 runs, 39 migrations.

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U aip -d aip --format=custom \
  --no-owner --no-privileges > "backups/aip-$(date -u +%Y%m%dT%H%M%SZ).dump"
# 452 KiB, 349 TOC entries

docker compose exec -T postgres pg_restore --list < backups/aip-*.dump
docker compose exec -T postgres psql -U aip -d postgres \
  -c 'CREATE DATABASE aip_restore_check'
docker compose exec -T postgres pg_restore -U aip -d aip_restore_check \
  --no-owner --no-privileges < backups/aip-*.dump
# orgs=12, runs=373, migrations=39
docker compose exec -T postgres psql -U aip -d postgres \
  -c 'DROP DATABASE aip_restore_check'
```

## Targets

| Metric | Target | Current |
|---|---|---|
| RPO | ≤ 24 h | manual backup; schedule not automated yet |
| RTO | ≤ 1 h | restore drill measured in minutes |
| Retention | 14 days | script default |
| Drill | monthly | verified once, needs a schedule |

## Gaps

- No automated backup schedule (cron/CronJob) or off-host storage yet.
- `backup.sh` needs `pg_dump` on the host; in this environment the dump was taken
  through `docker compose exec postgres`.
- Restore is logical and into a scratch database; production restore requires a
  maintenance window and an explicit approval.

## Rollback

The restore drill only creates/drops `aip_restore_check`; it never touches the
primary database. `git revert` of the documentation is safe.


## Re-drill after migration 69 (2026-11-04)

- source: platform database (schema version 69)
- `pg_dump -Fc` → 550,757 bytes
- restored into a scratch database with `pg_restore --no-owner --no-privileges --exit-on-error`
- counts matched: `orgs/projects/tasks/runs = 12/12/328/373`
- scratch database dropped; main database untouched


## Patroni backup drill (2026-11-05)

- source: `aip-patroni-patroni1-1` (Patroni primary, platform DB)
- `pg_dump -Fc` → 592,765 bytes
- restored into `aip_backup_test` with `template0` (template1 carries
  `metric_helpers`/`user_management`, which collided)
- counts: `12/12/330/374`; scratch DB dropped
- script: `scripts/patroni-backup-drill.sh`
