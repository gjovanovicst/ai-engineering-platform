# Patroni network partition drill (isolated)

Status: **drill implemented and run**. No platform state is used.

`scripts/patroni-partition.sh` builds two networks: `dcs` (etcd + Patroni) and
`client` (client visibility). It starts a leader and replica, then disconnects the
leader from the DCS network **while leaving the client network connected**.

## Verified run

```
leader=pg1
replica=pg2
partition=pg1-from-etcd
partitioned_role=master other_role=master partitioned_write=rejected
pg1_primary=503 pg1_replica=503 pg2_primary=200
patroni partition drill ok
```

- the partitioned node still reported `master` on `/patroni` (stale, expected)
- **writes on it were rejected**
- its `/primary` and `/replica` health checks both returned **503**
- the other node's `/primary` returned **200**

The important result for HAProxy: health routing uses `/primary` (503 for the
partitioned node), not the stale role field. A client routed by `/primary` goes to
the writable node.

## Not proven

- fencing at the host/kernel level (no watchdog)
- behaviour when both sides retain partial connectivity
- client-visible reconnect timing (HAProxy not in this script)
- production etcd TLS/auth
