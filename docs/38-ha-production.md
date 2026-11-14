# Production Postgres HA — design

Status: **design only**. The isolated failover drill is kept in the private operations repository.
proved manual promotion (RTO 1s, RPO 0 for the seeded row). This document defines
what "production HA" would require; none of it is deployed.

## 1. Topology

```
apps (N) ──> PgBouncer ──> HAProxy ──> Postgres primary (Patroni)
                                      └> synchronous replica (Patroni)
etcd x3 (leader election + DCS)          async replica (read/reporting)
```

- 3 etcd nodes (odd quorum), separate from Postgres hosts.
- Patroni on every Postgres node; one leader, at least one synchronous standby.
- HAProxy health-checks Patroni's `/primary` and `/replica` endpoints.
- PgBouncer in transaction mode in front; app DSN points at PgBouncer, never a
  Postgres hostname.
- Fencing: Patroni `pg_rewind` plus node/watchdog fencing; a demoted primary must
  not accept writes.

## 2. Failure behavior

| Failure | Expected |
|---|---|
| primary crash | Patroni promotes sync standby; HAProxy reroutes; writes resume |
| network partition | quorum side keeps leader; minority is fenced/read-only |
| replica lag | sync standby gates commits; alert if lag > threshold |
| PgBouncer loss | second instance behind HAProxy |
| etcd loss of quorum | no automatic failover; page on-call, do not force promote |

## 3. Application requirements

- DSN via PgBouncer/HAProxy with `target_session_attrs=read-write`.
- Bounded retries with jitter on transient connection errors; no retry on
  serialization/validation errors.
- Idempotency keys for enqueue/approval writes so a retry after failover cannot
  duplicate work.
- Readiness (`/readyz`) fails when the DB is in recovery/read-only; liveness (`/healthz`) stays process-local. Implemented: `writablePool` queries `pg_is_in_recovery()`. Compose uses `/readyz` for the API healthcheck and the worker metrics `/healthz`; both containers report `healthy`.
- Migrations only through one leader; advisory lock + version check.

## 4. Acceptance tests (must pass before claiming HA)

1. Kill primary under write load; measure RTO/RPO; assert no duplicate idempotent
   writes.
2. Partition the primary from etcd; assert fencing and no split-brain writes.
3. Sync replica lag > threshold; assert commits gate or alert.
4. PgBouncer kill; assert reconnect through HAProxy.
5. Full AZ loss drill with a documented runbook and timing.

## 5. What we have today

- single Postgres, no automatic failover
- manual promotion drill only (`scripts/ha-drill.sh`)
- PgBouncer pair + HAProxy are part of the default API/worker path; prepared
  statement tracking is enabled (`MAX_PREPARED_STATEMENTS=100`)
- HAProxy fronts two PgBouncer instances (host 6433 → container 6432); stopping
  one PgBouncer still served `orgs` through the other (live 12 → 12)
- auth rate limits are shared through Redis when `AIP_REDIS_ADDR` is set (live:
  6th bad login → 429); PgBouncer profile is verified
- no HAProxy/Patroni/etcd, no automatic app repoint
- writable `/readyz` and Compose healthchecks
- backup/restore drill in `docs/29-backup-restore.md`

Do not describe the platform as HA until the acceptance tests above are run and
recorded.
