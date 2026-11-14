# HA failover drill (isolated)

Status: **drill implemented and run**. This is not production HA automation.

`scripts/ha-drill.sh` creates two disposable Postgres 17 containers (primary +
streaming replica), seeds a row, promotes the replica after stopping the primary,
writes to the promoted node and tears everything down. It never touches the
platform database.

## Verified run

```
replication=streaming
rto_seconds=1 rows=2
ha drill ok
```

- replica had the pre-failover row (`before`)
- after `pg_ctl promote`, the promoted node accepted `after`
- RTO measured from `docker stop` to the first successful write: **1s**
- RPO for the seeded row: **0** (streaming, async replica)

## What this does not prove

- no automatic failover/leader election (promotion is manual)
- the application is not repointed; no PgBouncer/service discovery
- async replication can lose the tail under load (RPO > 0)
- no split-brain prevention, fencing or witness
- single-region only

## Reproduce

```bash
bash scripts/ha-drill.sh
```

Requires Docker and the `postgres:17-alpine`/`alpine` images. Containers, network
and volumes are removed by a trap.
