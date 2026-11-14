#!/usr/bin/env bash
# Take a wal-g base backup and record a heartbeat. Intended for cron.
set -euo pipefail
PG_CONTAINER=${PG_CONTAINER:-aip-patroni-patroni1-1}
LOCK=/tmp/aip-base-backup.lock

if ! mkdir "$LOCK" 2>/dev/null; then
    echo "base backup already running" >&2
    exit 1
fi
trap 'rmdir "$LOCK"' EXIT

name=$(docker exec -u postgres "$PG_CONTAINER" sh -c '
    set -a; . /home/postgres/pgdata/walg.env; set +a
    PGHOST=/var/run/postgresql PGUSER=postgres PGPORT=5432 \
      /usr/local/bin/wal-g backup-push /home/postgres/pgdata/pgroot/data >/dev/null
    set -a; . /home/postgres/pgdata/walg.env; set +a
    /usr/local/bin/wal-g backup-list | tail -1 | awk "{print \$1}"')
[ -n "$name" ] || { echo "no backup name returned" >&2; exit 1; }

docker exec "$PG_CONTAINER" psql -U postgres -d aip -v ON_ERROR_STOP=1 -c \
    "insert into backup_heartbeat(kind,last_success_at,detail) values ('base',now(),'$name')
     on conflict (kind) do update set last_success_at=excluded.last_success_at, detail=excluded.detail" >/dev/null
echo "base_backup=$name heartbeat=updated"
