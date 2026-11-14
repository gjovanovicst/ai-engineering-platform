# PITR drill with pg_receivewal

Status: **passed** on 2026-11-05. This is a streaming-WAL drill, not a production
archive setup.

## Why pg_receivewal

The live cluster's `archive_command` is `/bin/true`; a Patroni `/config` patch to
a local archive dir was not applied to PostgreSQL (docs/58). The drill therefore
streamed WAL with `pg_receivewal` on a physical replication slot.

## Steps

1. physical slot `aip_pitr`; `pg_receivewal` streamed WAL to a volume
2. `CREATE TABLE pitr_probe`; row `1=before-base`
3. `pg_basebackup -X stream -c fast` into a scratch volume
4. row `2` at `21:30:52.046`; row `3` at `21:30:55.265`; WAL switch
5. restore target **21:30:53.5** (between 2 and 3) with
   `recovery_target_time`, `restore_command = cp /wal/%f %p`,
   `recovery_target_action = promote`
6. started the copy with the vanilla `postgres:16-alpine` (trust HBA, SSL off,
   no Spilo preload libraries)

## Verified

```
restored log file "00000001000000000000001F" from archive
archive recovery complete
database system is ready to accept connections
1|before-base
2|after-base
```

**Row 3 is absent** — the restore stopped at the target timestamp, after row 2 and
before row 3.

## Cleanup

- replication slot dropped, receivewal stopped, scratch volumes removed
- the temporary `host replication postgres 172.28.0.0/16` HBA line was restored
  from `/tmp/pg_hba.before-pitr` and PostgreSQL reloaded
- `pitr_probe` dropped from the live database

## Limits

- streaming slot, not a durable WAL archive; no RPO claim
- standalone restore, not a Patroni rejoin
- target time was chosen manually; no automated PITR API
- no WAL retention/GCS/S3 archive policy yet
