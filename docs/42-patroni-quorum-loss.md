# Patroni behaviour on etcd quorum loss (isolated)

Status: **drill implemented and run**. No platform state is used.

`scripts/patroni-quorum-loss.sh` starts 3 etcd nodes, a Patroni leader and one
replica, then stops two etcd nodes and observes the leader.

## Verified run

```
leader=pg1
replica=pg2
etcd_quorum=lost
leader_role_after_45s=replica
writes_after_quorum_loss=rejected
patroni quorum-loss drill done
```

- with quorum, `pg1` was leader and `pg2` a replica
- stopping 2/3 etcd removed quorum
- after 45s, Patroni reported `pg1` as **replica** (demoted)
- a write on the demoted node was **rejected**

This is the safety property that matters: no writes continue on a former leader
that cannot renew its DCS lease.

## Not proven

- fencing at the host/kernel level (watchdog is not available in the container)
- split-brain under a network partition (as opposed to etcd processes stopping)
- how quickly clients/HAProxy observe the demotion
- TLS/auth and production etcd configuration
