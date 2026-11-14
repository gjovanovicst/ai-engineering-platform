# Patroni volume boot restore

Status: **run** on 2026-11-05.

`scripts/patroni-volume-boot-restore.sh` extends the cold volume drill: after
capturing the stopped replica, it extracts the archive into a scratch volume and
boots it as a standalone PostgreSQL 16 to read real data.

## Verified run

```
archive_bytes=10190561
api_during_stop=200
restored_counts=12/12/330/374
patroni volume boot restore ok
```

- the archive was extracted into a clean volume
- the data directory was started with the vanilla `postgres:16-alpine` image
- Spilo-specific settings were overridden for the drill:
  `shared_preload_libraries=''`, `ssl=off`, and a local trust HBA
  (the Spilo HBA uses `hostssl`/PAM, which the vanilla image lacks)
- real counts were read from the restored database

## Limits

- standalone boot, not a Patroni cluster rejoin
- `standby.signal` was removed; the restored copy was opened read/write for the
  drill and then destroyed
- cold backup (replica stopped); API stayed up
- no PITR/WAL replay test
