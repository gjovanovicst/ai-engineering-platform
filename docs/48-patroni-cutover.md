# Platform cutover to Patroni (executed)

Status: **executed in the development stack** on 2026-11-05. The old Postgres is
still running and retains the pre-cutover data for rollback.

## Deployment

`deploy/patroni/compose.yml` is a separate Compose project (`aip-patroni`) on the
external `aip_default` network:

- one etcd (v2 API enabled), two Spilo 16 nodes, HAProxy with `/primary` checks
- named volumes for etcd and both nodes
- HAProxy alias `patroni-haproxy:5432`, host port `15432`

Start it before the main stack, then materialize data:

```
docker compose -f deploy/patroni/compose.yml up -d
```

The platform points at it through `.env`:

```
AIP_DB_DSN=postgres://aip_app:app@patroni-haproxy:5432/aip?sslmode=require
```

## Verified

```
counts=12/12/330/374            # orgs/projects/tasks/runs
readyz=200
login=200
projects=200
pg_stat_activity aip_app=6      # API/worker connections on patroni1
```

(`330/374` includes two test tasks created during the plugin drill.)

## Rollback

1. remove/restore `AIP_DB_DSN` in `.env` to the old HAProxy/PgBouncer DSN
2. `docker compose up -d --force-recreate api worker`
3. keep `aip-patroni` stopped; do not delete its volumes until the window closes

Writes after the cutover go only to Patroni; rolling back discards them. The old
database is a **pre-cutover snapshot**, not a replica.

## Not covered

- sync replication / zero data loss: `synchronous_mode` left a stale
  `/service/aip-ha/sync` key (`sync_standby:null`); deleting it promoted the
  survivor, so automatic sync failover is not safe here. The platform runs async
- host fencing and automatic client failover to a new primary (HAProxy handles
  node health, not split-brain fencing)
- volume-level backups: a cold replica drill captures the volume and boots it
  standalone (docs/50/51); it is not re-joined as a Patroni node
- production etcd TLS/auth


## Static IPs after Docker restarts (2026-11-10)

Docker restarts changed the node IPs while Patroni's DCS still held the old ones;
the replica stopped streaming and HAProxy failed when `patroni2` DNS disappeared.

- Patroni nodes now get static IPs `172.28.0.18` (leader) and `.17` (replica) on
  `aip_default`
- HAProxy backends use those IPs, not hostnames, so it starts with the replica
  down
- rebuilt the replica from the leader; it is streaming again
