# Recovering a stale Patroni sync key

Status: **manual runbook/tool, not automatic**. Sync mode is **off** on the
platform cluster.

## Symptom

After the sync-mode leader dies, the survivor logs:

```
following a different leader because i am not the healthiest node
```

and does not promote. `/service/<scope>/sync` still contains the old leader with
`sync_standby:null`; deleting that key promoted the survivor in the drill within
~10s.

## Safety gates (all required)

1. the old primary is **down or fenced** (`PRIMARY_DOWN=yes`)
2. the etcd leader key is absent (`/service/<scope>/leader`)
3. the expected survivor member key exists
4. the operator confirms the scope (`CONFIRM=<scope>`)
5. the survivor is the only writable candidate; never run with two live nodes

If any gate fails, the script refuses. This is deliberate: deleting the sync key
while the old primary can still write risks split-brain.

## Run

```
ETCD=aip-patroni-etcd-1 SCOPE=aip-ha SURVIVOR=<member> \
  PRIMARY_DOWN=yes CONFIRM=aip-ha \
  scripts/patroni-sync-recover.sh
```

Then:

- verify the survivor is `/primary` 200 and `pg_is_in_recovery()=f`
- check `pg_stat_replication` for the returning node
- investigate why Patroni left the sync key (version/behavior) before enabling
  sync mode again

## Not solved

- automatic cleanup (would need fencing and a race-free design)
- split-brain prevention without host fencing
- enabling `synchronous_mode` on the platform
