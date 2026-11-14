# Restore from the offsite MinIO

Status: **passed** on 2026-11-11.

## Method

`OFFSITE=1` makes the restore runner rewrite the copied `walg.env` endpoint to
`http://minio-offsite:9000`; the rest is the normal runner (latest base, target,
scratch PostgreSQL).

A failed attempt had left data in the scratch volume; the runner now wipes the
volume before `wal-g backup-fetch` and records `backup fetch failed` on error.

## Verified

```
request target 2026-11-10 23:29:28+00
base_backup=base_00000003000000000000003D
restored counts=12/12/330
status=succeeded
```

This used only the offsite MinIO endpoint (source data not required), so the
mirror is a usable recovery source, not just a copy.

## Limits

- same host / separate container and volume; not geographic
- offsite mirror is full-bucket with `--remove`
- restore is a scratch instance, not a live cutover
- offsite credentials are still the MinIO alias config
