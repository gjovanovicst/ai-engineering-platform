# Patroni replica maintenance drill

Status: **run** on 2026-11-05 against the platform Patroni cluster.

`scripts/patroni-replica-drill.sh` stops the replica, checks the API and a write
path, restarts the replica and waits for streaming replication.

## Verified run

```
before: Leader running, Replica streaming, lag 0 MB
during_stop api=200 orgs=12
rejoined recovery=t replay_lag=0.001484
after: Leader running, Replica streaming, lag 0 MB
api=200
```

The API stayed up while the only replica was down. HAProxy health-checks the
primary; no failover was needed.

## Not covered

- stopping the primary (failover already covered by docs/37/40)
- volume-level backup of the stopped replica
- long partitions; the replica re-streamed from the primary
