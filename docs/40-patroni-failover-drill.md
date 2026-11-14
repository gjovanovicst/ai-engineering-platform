# Patroni/etcd failover drill (isolated)

Status: **drill implemented and run**. This is not the production deployment.

`scripts/patroni-drill.sh` creates a disposable network with:

- etcd 3.5 (single node, v2 API enabled for Spilo's compatibility)
- two Spilo/Patroni PostgreSQL 16 nodes (one leader, one streaming replica)
- HAProxy on the client port, health-checking Patroni `/primary` on each node
- promotion + write check through HAProxy after stopping the leader

## Verified run

```
leader=pg1
replica=pg2
promoted=pg2 rto_seconds=13 rows=2
patroni drill ok
```

- Patroni elected `pg1`, `pg2` cloned and followed
- stopping `pg1` caused Patroni to promote `pg2` automatically
- HAProxy health checks moved traffic to `pg2`; a psql client through HAProxy
  accepted a write and returned both seeded rows
- RTO measured from `docker stop` to a successful client connection through
  HAProxy: **13s** (Patroni promotion + HAProxy health check)

## Not proven

- the platform API/worker point at HAProxy/PgBouncer; the drill client uses HAProxy.
  The Patroni cluster itself is still not the platform database
- no etcd quorum test (single etcd in the drill)
- no fencing/watchdog (Spilo logged the missing `/dev/watchdog`)
- no synchronous replication / RPO measurement under load. Attempts to enable
  `synchronous_mode` did not promote after 180s; root cause found in etcd:
  `/service/aip-ha/sync` stayed behind with `{"leader":"<old>","sync_standby":null}`.
  Deleting that key made the surviving node leader within ~10s. The platform
  therefore keeps async replication; sync RPO is unproven and the manual recovery gate is in
  [53-patroni-sync-recovery.md](53-patroni-sync-recovery.md); sync mode stays off
- no production TLS, backups, or resource limits
- disposable containers are removed by a trap

## Reproduce

```bash
bash scripts/patroni-drill.sh
```

Requires `ghcr.io/zalando/spilo-16:3.3-p1`, `quay.io/coreos/etcd:v3.5.15`,
`curlimages/curl` and `postgres:17-alpine` images.
