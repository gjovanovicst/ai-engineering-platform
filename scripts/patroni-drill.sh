#!/usr/bin/env bash
# Isolated Patroni + etcd failover drill. No platform data is used.
set -euo pipefail
SUFFIX="$$"
NET="aip-patroni-${SUFFIX}"
ETCD="aip-etcd-${SUFFIX}"
PG1="aip-pg1-${SUFFIX}"
PG2="aip-pg2-${SUFFIX}"
HAPROXY="aip-haproxy-${SUFFIX}"

cleanup() {
    docker rm -f "$PG1" "$PG2" "$ETCD" "$HAPROXY" >/dev/null 2>&1 || true
    rm -f /tmp/patroni-haproxy.cfg
    docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
docker run -d --name "$ETCD" --network "$NET" quay.io/coreos/etcd:v3.5.15 \
    /usr/local/bin/etcd --name etcd --enable-v2=true \
    --advertise-client-urls "http://$ETCD:2379" --listen-client-urls http://0.0.0.0:2379 \
    --initial-advertise-peer-urls "http://$ETCD:2380" --listen-peer-urls http://0.0.0.0:2380 \
    --initial-cluster "etcd=http://$ETCD:2380" --initial-cluster-state new >/dev/null
docker run -d --name "$PG1" --network "$NET" \
    -e SCOPE=aip-ha -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg1 -e POD_IP="$PG1" -e PGPORT=5432 -e PGPASSWORD=postgres \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null

for _ in $(seq 1 60); do
    role=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role" = "master" ] && break
    sleep 2
done
[ "$role" = "master" ] || { echo "leader did not start" >&2; exit 1; }
echo "leader=pg1"

docker run -d --name "$PG2" --network "$NET" \
    -e SCOPE=aip-ha -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg2 -e POD_IP="$PG2" -e PGPORT=5432 -e PGPASSWORD=postgres \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
for _ in $(seq 1 90); do
    role2=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role2" = "replica" ] && break
    sleep 2
done
[ "$role2" = "replica" ] || { echo "replica did not start" >&2; exit 1; }
echo "replica=pg2"

docker exec "$PG1" psql -q -U postgres -d postgres -c "alter user postgres password 'postgres'" >/dev/null

cat > /tmp/patroni-haproxy.cfg <<CFG
global
  daemon
defaults
  mode tcp
  timeout connect 3s
  timeout client 30s
  timeout server 30s
frontend pg
  bind *:5432
  default_backend pg
backend pg
  option httpchk GET /primary
  http-check expect status 200
  server pg1 $PG1:5432 check port 8008
  server pg2 $PG2:5432 check port 8008
CFG
docker run -d --name "$HAPROXY" --network "$NET" -v /tmp/patroni-haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro haproxy:3.0-alpine >/dev/null
sleep 3
echo "haproxy=up"

docker exec "$PG1" psql -U postgres -d postgres -tAc \
    "create table drill(id int); insert into drill values (1)" >/dev/null

start=$(date +%s)
docker stop "$PG1" >/dev/null
for _ in $(seq 1 90); do
    role2=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role2" = "master" ] && break
    sleep 2
done
[ "$role2" = "master" ] || { echo "promotion failed" >&2; exit 1; }
for _ in $(seq 1 90); do
    if docker run --rm --network "$NET" -e PGPASSWORD=postgres postgres:17-alpine \
        psql "host=$HAPROXY user=postgres dbname=postgres sslmode=require" -tAc "select 1" 2>/dev/null | grep -qx 1; then
        break
    fi
    sleep 1
done
end=$(date +%s)
docker run --rm --network "$NET" -e PGPASSWORD=postgres postgres:17-alpine \
    psql "host=$HAPROXY user=postgres dbname=postgres sslmode=require" -q -c "insert into drill values (2)" >/dev/null
rows=$(docker run --rm --network "$NET" -e PGPASSWORD=postgres postgres:17-alpine \
    psql "host=$HAPROXY user=postgres dbname=postgres sslmode=require" -tAc "select count(*) from drill")
echo "promoted=pg2 rto_seconds=$((end-start)) rows=$rows"
[ "$rows" = "2" ] || { echo "write on promoted leader failed" >&2; exit 1; }
echo "patroni drill ok"
