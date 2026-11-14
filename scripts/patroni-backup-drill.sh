#!/usr/bin/env bash
# Backup/restore drill against the Patroni-backed platform database.
set -euo pipefail
PRIMARY=${PRIMARY:-aip-patroni-patroni1-1}
DUMP=/tmp/aip-patroni-backup.dump

docker exec "$PRIMARY" pg_dump -U postgres -Fc aip > "$DUMP"
echo "dump_bytes=$(stat -c %s "$DUMP")"
docker cp "$DUMP" "$PRIMARY:/tmp/aip-backup.dump" >/dev/null
docker exec "$PRIMARY" psql -q -U postgres -d postgres -c "drop database if exists aip_backup_test" >/dev/null
docker exec "$PRIMARY" psql -q -U postgres -d postgres -c "create database aip_backup_test template template0" >/dev/null
docker exec "$PRIMARY" pg_restore -U postgres -d aip_backup_test --no-owner --no-privileges --exit-on-error /tmp/aip-backup.dump >/dev/null
counts=$(docker exec "$PRIMARY" psql -U postgres -d aip_backup_test -tAc "select (select count(*) from orgs)||'/'||(select count(*) from projects)||'/'||(select count(*) from tasks)||'/'||(select count(*) from runs)")
echo "restored_counts=$counts"
docker exec "$PRIMARY" psql -q -U postgres -d postgres -c "drop database aip_backup_test" >/dev/null
rm -f "$DUMP"
echo "patroni backup drill ok"
