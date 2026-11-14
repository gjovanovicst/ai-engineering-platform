# Patroni replica volume backup drill

Status: **run** on 2026-11-05.

`scripts/patroni-volume-backup.sh` stops the replica, tars its cold data volume,
restarts the replica, extracts the archive into a scratch volume and validates
the PostgreSQL control file.

## Verified run

```
archive_bytes=10178561
api_during_stop=200
pg_control version number:            1300
Catalog version number:               202307071
Database system identifier:           7704623835753918522
Database cluster state:               shut down in recovery
patroni volume backup drill ok
```

- the API stayed up with the replica stopped
- the archive extracted into a clean volume
- `pg_controldata` read a valid PG16 data directory (same system identifier)

## Limits

- a follow-up boots the archive standalone and reads counts (docs/51); this is
  still not a Patroni cluster rejoin
- cold backup only (replica stopped); no volume snapshot/consistency guarantees
  for a running primary
- the old Postgres logical backup in docs/29 remains the application-level
  restore path
