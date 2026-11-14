# PITR attempt — blocked by Spilo/config reload

Status: **not implemented**, 2026-11-05.

## What was tried

1. created `/home/postgres/pgdata/archive` on the Patroni leader
2. `PATCH /config` set:
   `archive_command = test ! -f .../archive/%f && cp %p .../archive/%f`
3. forced `patronictl reload`

## Result

```
DCS config: archive_command = cp ...
PostgreSQL: show archive_command = /bin/true
pending_restart = f
archive files = 0
```

Patroni accepted the DCS value but PostgreSQL kept the old command; the reload
did not apply it. The DCS change was reverted to `/bin/true` so no drift remains.

## What a real PITR slice needs

- a cluster where `archive_command` is applied (bootstrap config or a controlled
  restart), **or** a `pg_receivewal` sidecar with a replication slot
- a base backup (`pg_basebackup -X stream`), WAL archive, and a restore host
- a target timestamp between two marker writes; assert the later marker is absent
- a tested restore time and a documented RPO

Do not claim PITR until that restore is executed.
