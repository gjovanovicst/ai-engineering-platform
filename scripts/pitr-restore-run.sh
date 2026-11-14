#!/usr/bin/env bash
# Execute the oldest pending restore request against a scratch PostgreSQL.
set -eu
PG_CONTAINER=${PG_CONTAINER:-aip-patroni-patroni1-1}
REQUEST_ID=${REQUEST_ID:-}
WORK=/tmp/aip-restore-$$
ENV_FILE=/tmp/aip-restore-walg.env
VOLUME=aip-restore-scratch
CONTAINER=aip-restore-run

cleanup() {
    docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    docker volume rm "$VOLUME" >/dev/null 2>&1 || true
    rm -f "$ENV_FILE"
    rm -rf "$WORK"
}
trap cleanup EXIT

if [ -n "$REQUEST_ID" ]; then
    row=$(docker exec "$PG_CONTAINER" psql -U postgres -d aip -tAc \
        "update restore_requests set status='running', updated_at=now()
         where id='$REQUEST_ID' and status='pending'
         returning id||'|'||to_char(target_time at time zone 'UTC','YYYY-MM-DD HH24:MI:SS+00')" | head -1)
else
    row=$(docker exec "$PG_CONTAINER" psql -U postgres -d aip -tAc \
        "update restore_requests set status='running', updated_at=now()
         where id=(select id from restore_requests where status='pending' order by created_at limit 1)
         returning id||'|'||to_char(target_time at time zone 'UTC','YYYY-MM-DD HH24:MI:SS+00')" | head -1)
fi
[ -n "$row" ] || { echo "no pending restore request"; exit 0; }
ID=${row%%|*}; TARGET=${row#*|}
echo "request=$ID target=$TARGET"

before=$(docker exec "$PG_CONTAINER" psql -U postgres -d aip -tAc "select last_archived_wal from pg_stat_archiver" 2>/dev/null || true)
docker exec "$PG_CONTAINER" psql -U postgres -d aip -c "select pg_switch_wal()" >/dev/null 2>&1 || true
for _ in $(seq 1 15); do
    now=$(docker exec "$PG_CONTAINER" psql -U postgres -d aip -tAc "select last_archived_wal from pg_stat_archiver" 2>/dev/null || true)
    [ "$now" != "$before" ] && break
    sleep 1
done

backup=$(docker exec -u postgres "$PG_CONTAINER" sh -c '
    set -a; . /home/postgres/pgdata/walg.env; set +a
    /usr/local/bin/wal-g backup-list | tail -1 | awk "{print \$1}"')
[ -n "$backup" ] || { docker exec "$PG_CONTAINER" psql -U postgres -d aip -c "update restore_requests set status='failed', detail='no base backup', updated_at=now() where id='$ID'" >/dev/null; exit 1; }
echo "base_backup=$backup"

docker cp "$PG_CONTAINER:/home/postgres/pgdata/walg.env" "$ENV_FILE"
if [ "${OFFSITE:-0}" = "1" ]; then
    sed -i 's#^AWS_ENDPOINT=.*#AWS_ENDPOINT=http://minio-offsite:9000#' "$ENV_FILE"
fi
docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
docker volume rm "$VOLUME" >/dev/null 2>&1 || true
docker volume create "$VOLUME" >/dev/null
docker run --rm -v "$VOLUME":/data alpine:3.20 sh -c 'rm -rf /data/* /data/.[!.]*' >/dev/null
if ! docker run --rm --network aip_default -v "$VOLUME":/data --env-file "$ENV_FILE" \
    --entrypoint /usr/local/bin/wal-g ghcr.io/zalando/spilo-16:3.3-p1 \
    backup-fetch /data "$backup" >/dev/null 2>&1; then
    docker exec "$PG_CONTAINER" psql -U postgres -d aip -c \
        "update restore_requests set status='failed', detail='backup fetch failed', updated_at=now() where id='$ID'" >/dev/null
    echo "backup fetch failed" >&2
    exit 1
fi
docker run --rm -v "$VOLUME":/data alpine:3.20 sh -c "chown -R 101:103 /data && chmod 700 /data && \
    printf \"\\nrestore_command = '/usr/local/bin/wal-g wal-fetch %%f %%p'\nrecovery_target_time = '$TARGET'\nrecovery_target_timeline = '3'\nrecovery_target_action = 'promote'\nrecovery_target_inclusive = on\n\" >> /data/postgresql.auto.conf && \
    touch /data/recovery.signal && \
    printf 'local all all trust\nhost all all 127.0.0.1/32 trust\n' > /data/pg_hba.test.conf && \
    chown 101:103 /data/recovery.signal /data/pg_hba.test.conf /data/postgresql.auto.conf"

docker run -d --name "$CONTAINER" --network aip_default --user 101:103 -v "$VOLUME":/data \
    --env-file "$ENV_FILE" -e PGDATA=/data --entrypoint postgres ghcr.io/zalando/spilo-16:3.3-p1 \
    -D /data -c listen_addresses='' -c port=5436 -c unix_socket_directories=/tmp \
    -c shared_preload_libraries='' -c ssl=off -c hba_file=/data/pg_hba.test.conf \
    -c ident_file=/data/pg_ident.conf -c logging_collector=off >/dev/null

counts=""
for _ in $(seq 1 60); do
    counts=$(docker exec "$CONTAINER" psql -h /tmp -p 5436 -U postgres -d aip -tAc \
        "select (select count(*) from orgs)||'/'||(select count(*) from projects)||'/'||(select count(*) from tasks)" 2>/dev/null || true)
    [ -n "$counts" ] && break
    sleep 2
done
if [ -n "$counts" ]; then
    docker exec "$PG_CONTAINER" psql -U postgres -d aip -c \
        "update restore_requests set status='succeeded', detail='restored at $TARGET; orgs/projects/tasks=$counts', updated_at=now() where id='$ID'" >/dev/null
    docker exec "$PG_CONTAINER" psql -U postgres -d aip -c \
        "insert into backup_heartbeat(kind,last_success_at,detail) values ('restore',now(),'request $ID; $counts')
         on conflict (kind) do update set last_success_at=excluded.last_success_at, detail=excluded.detail" >/dev/null
    echo "restored counts=$counts"
else
    docker exec "$PG_CONTAINER" psql -U postgres -d aip -c \
        "update restore_requests set status='failed', detail='scratch restore did not become ready', updated_at=now() where id='$ID'" >/dev/null
    echo "scratch restore failed" >&2
    docker logs --tail 20 "$CONTAINER" >&2 || true
    exit 1
fi
