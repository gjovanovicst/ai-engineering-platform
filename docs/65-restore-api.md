# Restore requests + DR dashboard

Status: **implemented and exercised** on 2026-11-10.

## API

- `GET /api/v1/orgs/{org}/dr` — WAL archived/failed/age, base age/name
- `POST /api/v1/orgs/{org}/restores` — admin queues `{target_time}` (RFC3339)
- `GET /api/v1/orgs/{org}/restores` — status list

Migration 0075 adds `restore_requests` (RLS by org). The API never restores the
live database; it queues a request and the operator runner executes it.

## Runner

`scripts/pitr-restore-run.sh`:

1. claims the oldest pending request (or `REQUEST_ID`)
2. reads the latest base backup via `wal-g backup-list`
3. fetches it into a scratch volume
4. starts a standalone Spilo PostgreSQL with the requested
   `recovery_target_time`
5. on success records counts; on failure records `failed` + detail

## Verified

```
POST /restores -> pending
runner: base_backup=base_00000003000000000000003D
restored counts=12/12/330
status: succeeded
detail: restored at 2026-11-10 22:20:18+00; orgs/projects/tasks=12/12/330
```

UI: Settings → Backups shows WAL/base health, queues a restore and lists status.

## DR SLO panel

`GET /dr` also returns `restore_succeeded`, `restore_failed`,
`last_restore_age_seconds` and `avg_restore_seconds` (RLS-scoped query). Live after
the drill: succeeded=1, failed=1, last age ~991s, avg restore ~874s.

## Important fix

WAL scripts/env were in the container filesystem; a Patroni recreate deleted them
and **archiving stopped**. They now live in the data volume:

- `/home/postgres/pgdata/walg.env`
- `/home/postgres/pgdata/archive-wal.sh`

`archive_command` and the backup/runner scripts use that path.

## Limits

- worker-driven path exists (`restore-worker`, docs/66); the manual runner remains
- restore is a scratch instance, not a live cutover
- no automatic verification query beyond counts
- target must be after the copied base and covered by archived WAL; a WAL gap is
  reported as failed


## Restore SLO alert

The runner records a `restore` heartbeat in `backup_heartbeat` on success.

`/metrics`:

```
aip_restore_last_success_age_seconds ...
```

Prometheus: `AIPRestoreStale` when no successful restore for 30 days.
