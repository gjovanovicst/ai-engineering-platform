# etcd quorum drill (isolated)

Status: **drill implemented and run**. No platform state is used.

`scripts/etcd-quorum-drill.sh` starts three etcd 3.5 nodes with a shared initial
cluster, then stops nodes and checks `etcdctl endpoint health --cluster`.

## Verified run

```
healthy_before=3
healthy_after_one_down=2
healthy_after_two_down=0
healthy_after_restart=3
etcd quorum drill ok
```

- 3/3 healthy → stopping one keeps 2/3 healthy
- stopping a second loses quorum: 0/3 endpoints report healthy
- restarting the two nodes restores 3/3

## Interpretation

With two of three etcd members down, Patroni's DCS has no quorum: it must not
elect a new leader and a leader must eventually demote/stop writes after its
lease expires. This drill verifies the etcd health signal only; it does not run
Patroni across the outage.

Not proven: fencing, Patroni demotion timing, split-brain behavior, production
TLS/auth, or application behavior during quorum loss.
