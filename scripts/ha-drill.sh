#!/usr/bin/env bash
# Isolated Postgres streaming-replication failover drill. No platform data is used.
set -euo pipefail

SUFFIX="$$"
PRIMARY="aip-ha-primary-${SUFFIX}"
REPLICA="aip-ha-replica-${SUFFIX}"
NET="aip-ha-net-${SUFFIX}"
VOL1="aip-ha-primary-${SUFFIX}"
VOL2="aip-ha-replica-${SUFFIX}"

cleanup() {
    docker rm -f "$PRIMARY" "$REPLICA" >/dev/null 2>&1 || true
    docker network rm "$NET" >/dev/null 2>&1 || true
    docker volume rm "$VOL1" "$VOL2" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
docker volume create "$VOL1" >/dev/null
docker volume create "$VOL2" >/dev/null
docker run -d --name "$PRIMARY" --network "$NET" -v "$VOL1":/var/lib/postgresql/data \
    -e POSTGRES_PASSWORD=drill -e POSTGRES_DB=aip postgres:17-alpine >/dev/null
for _ in $(seq 1 30); do docker exec "$PRIMARY" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
docker exec "$PRIMARY" psql -U postgres -c "create role replicator login replication password 'repl'" >/dev/null
docker exec "$PRIMARY" sh -c "echo 'host replication replicator 0.0.0.0/0 scram-sha-256' >> /var/lib/postgresql/data/pg_hba.conf"
docker exec -u postgres "$PRIMARY" pg_ctl reload -D /var/lib/postgresql/data >/dev/null

docker run --rm -v "$VOL2":/data alpine sh -c 'chown -R 70:70 /data; chmod 700 /data'
docker run -d --name "$REPLICA" --network "$NET" -v "$VOL2":/data -e PGDATA=/data -e PGPASSWORD=repl \
    postgres:17-alpine sh -c "until pg_isready -h $PRIMARY -U postgres; do sleep 1; done; rm -rf /data/*; pg_basebackup -h $PRIMARY -U replicator -D /data -Fp -Xs -R -P; chown -R 70:70 /data; chmod 700 /data; gosu postgres postgres -D /data" >/dev/null
for _ in $(seq 1 30); do
    state=$(docker exec "$PRIMARY" psql -U postgres -tAc "select state from pg_stat_replication" 2>/dev/null || true)
    [ "$state" = "streaming" ] && break
    sleep 1
done
[ "$state" = "streaming" ] || { echo "replication did not start" >&2; exit 1; }
echo "replication=streaming"

docker exec "$PRIMARY" psql -U postgres -d aip -c "create table drill(id int, note text); insert into drill values (1,'before')" >/dev/null
sleep 2
pre=$(docker exec "$REPLICA" psql -U postgres -d aip -tAc "select note from drill where id=1")
[ "$pre" = "before" ] || { echo "replica missing pre-failover row" >&2; exit 1; }

start=$(date +%s)
docker stop "$PRIMARY" >/dev/null
docker exec -u 70 "$REPLICA" pg_ctl promote -D /data >/dev/null
for _ in $(seq 1 30); do
    docker exec "$REPLICA" psql -U postgres -d aip -tAc "insert into drill values (2,'after')" >/dev/null 2>&1 && break
    sleep 1
done
end=$(date +%s)
rows=$(docker exec "$REPLICA" psql -U postgres -d aip -tAc "select count(*) from drill")
echo "rto_seconds=$((end-start)) rows=$rows"
[ "$rows" = "2" ] || { echo "failover write failed" >&2; exit 1; }
echo "ha drill ok"
