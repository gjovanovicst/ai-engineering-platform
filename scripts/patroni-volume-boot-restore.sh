#!/usr/bin/env bash
# Cold replica volume backup + boot the restored data dir as a standalone PG16.
set -euo pipefail
REPLICA=${REPLICA:-aip-patroni-patroni2-1}
VOLUME=${VOLUME:-aip-patroni_patroni2-data}
TEST_VOLUME=${TEST_VOLUME:-aip-boot-test}
BOOT_CONTAINER=${BOOT_CONTAINER:-aip-boot-test}
ARCHIVE=/tmp/patroni-boot.tgz
API=${API:-http://localhost:18080/readyz}

cleanup() {
    docker rm -f "$BOOT_CONTAINER" >/dev/null 2>&1 || true
    docker volume rm "$TEST_VOLUME" >/dev/null 2>&1 || true
    docker run --rm -v /tmp:/tmp alpine:3.20 rm -f "$ARCHIVE" >/dev/null 2>&1 || true
}
trap cleanup EXIT

owner=$(docker exec "$REPLICA" stat -c '%u:%g' /home/postgres/pgdata/pgroot/data)
docker stop "$REPLICA" >/dev/null
docker run --rm -v "$VOLUME":/data:ro -v /tmp:/backup alpine:3.20 tar czf "/backup/$(basename "$ARCHIVE")" -C /data .
echo "archive_bytes=$(stat -c %s "$ARCHIVE")"
code=$(curl -s -o /dev/null -m 5 -w '%{http_code}' "$API")
echo "api_during_stop=$code"
[ "$code" = "200" ] || { docker start "$REPLICA" >/dev/null; echo "api failed during stop" >&2; exit 1; }
docker start "$REPLICA" >/dev/null

docker volume create "$TEST_VOLUME" >/dev/null
docker run --rm -v "$TEST_VOLUME":/data -v /tmp:/backup alpine:3.20 sh -c "tar xzf /backup/$(basename "$ARCHIVE") -C /data && rm -f /data/pgroot/data/standby.signal /data/pgroot/data/postmaster.pid && printf 'local all all trust\nhost all all 127.0.0.1/32 trust\nhost all all ::1/128 trust\n' > /data/pgroot/data/pg_hba.test.conf"

docker run -d --name "$BOOT_CONTAINER" --user "$owner" -v "$TEST_VOLUME":/data -e PGDATA=/data/pgroot/data \
    postgres:16-alpine postgres -D /data/pgroot/data \
    -c listen_addresses='' -c port=5433 -c unix_socket_directories=/tmp \
    -c shared_preload_libraries='' -c ssl=off \
    -c hba_file=/data/pgroot/data/pg_hba.test.conf -c ident_file=/data/pgroot/data/pg_ident.conf >/dev/null

for _ in $(seq 1 30); do
    counts=$(docker exec "$BOOT_CONTAINER" psql -h /tmp -p 5433 -U postgres -d aip -tAc \
        "select (select count(*) from orgs)||'/'||(select count(*) from projects)||'/'||(select count(*) from tasks)||'/'||(select count(*) from runs)" 2>/dev/null || true)
    [ -n "$counts" ] && break
    sleep 1
done
[ -n "$counts" ] || { echo "standalone boot did not answer" >&2; docker logs --tail 10 "$BOOT_CONTAINER" >&2 || true; exit 1; }
echo "restored_counts=$counts"
echo "patroni volume boot restore ok"
