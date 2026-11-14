#!/usr/bin/env bash
# Mirror the WAL/base bucket to the offsite MinIO endpoint and record a heartbeat.
set -euo pipefail
SOURCE=${SOURCE:-aip-minio-1}
LOCK=/tmp/aip-offsite-backup.lock
if ! mkdir "$LOCK" 2>/dev/null; then echo "offsite backup already running" >&2; exit 1; fi
trap 'rmdir "$LOCK"' EXIT

docker exec "$SOURCE" mc mirror --overwrite --remove local/aip-wal offsite/aip-wal >/dev/null
local_count=$(docker exec "$SOURCE" mc ls --recursive local/aip-wal | wc -l | tr -d ' ')
offsite_count=$(docker exec "$SOURCE" mc ls --recursive offsite/aip-wal | wc -l | tr -d ' ')
[ "$local_count" = "$offsite_count" ] || { echo "count mismatch local=$local_count offsite=$offsite_count" >&2; exit 1; }

docker exec aip-patroni-patroni1-1 psql -U postgres -d aip -c \
    "insert into backup_heartbeat(kind,last_success_at,detail) values ('offsite',now(),'objects=$offsite_count')
     on conflict (kind) do update set last_success_at=excluded.last_success_at, detail=excluded.detail" >/dev/null
echo "offsite_objects=$offsite_count heartbeat=updated"
