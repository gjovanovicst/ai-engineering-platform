# Patroni cutover rehearsal (isolated)

Status: **rehearsal implemented and run**. The main database was only read with
`pg_dump`; it was not modified.

`scripts/patroni-cutover-rehearsal.sh` starts a disposable Patroni cluster,
dumps the platform database, restores it into the cluster, runs migrations,
starts a temporary API against HAProxy/Patroni and verifies login.

## Verified run

```
dump_bytes=1661045
restored
counts_orgs/projects/tasks/runs=12/12/328/373
readyz=200
login=200
cutover rehearsal ok
```

- real platform data was restored (12/12/328/373)
- migrations reported no pending work (version 69)
- the temporary API served `/readyz` and authenticated the demo user
- the disposable cluster/API were removed by the trap

## Compatibility notes found by the rehearsal

- source is PostgreSQL 17, Spilo node is 16: the plain dump needed
  `SET transaction_timeout` and `\restrict`/`\unrestrict` removed
- migrations must run as the superuser (or the restored objects must be
  re-owned) because `goose_db_version` was owned by `postgres`, not the migrator
- `aip_app` grants must be applied to all restored/new tables after migrations

## Not covered

- zero-downtime cutover (writes are not replicated)
- rollback timing and DNS/connection draining
- production TLS/auth for etcd/Postgres
- app workers (only the API was started)
