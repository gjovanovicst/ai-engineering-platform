#!/usr/bin/env bash
# Rehearse the platform cutover to Patroni on a disposable cluster.
# The main database is only read with pg_dump; it is never modified.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOCACHE=${GOCACHE:-$ROOT/.cache/go-build}
export GOMODCACHE=${GOMODCACHE:-$ROOT/.cache/gomod}
export GOPATH=${GOPATH:-$ROOT/.cache/gopath}
SUFFIX="$$"
NET="aip-cut-${SUFFIX}"
ETCD="aip-cut-etcd-${SUFFIX}"
PG1="aip-cut-pg1-${SUFFIX}"
PG2="aip-cut-pg2-${SUFFIX}"
HAP="aip-cut-hap-${SUFFIX}"
API="aip-cut-api-${SUFFIX}"
DUMP=/tmp/aip-cutover-${SUFFIX}.sql

cleanup() {
    docker rm -f "$API" "$HAP" "$PG2" "$PG1" "$ETCD" >/dev/null 2>&1 || true
    docker network rm "$NET" >/dev/null 2>&1 || true
    rm -f "$DUMP"
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
docker run -d --name "$ETCD" --network "$NET" quay.io/coreos/etcd:v3.5.15 \
    /usr/local/bin/etcd --name etcd --enable-v2=true \
    --advertise-client-urls "http://$ETCD:2379" --listen-client-urls http://0.0.0.0:2379 \
    --initial-advertise-peer-urls "http://$ETCD:2380" --listen-peer-urls http://0.0.0.0:2380 \
    --initial-cluster "etcd=http://$ETCD:2380" --initial-cluster-state new >/dev/null
docker run -d --name "$PG1" --network "$NET" \
    -e SCOPE=aip-cut -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg1 -e POD_IP="$PG1" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
for _ in $(seq 1 90); do
    role=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role" = "master" ] && break
    sleep 2
done
[ "$role" = "master" ] || { echo "no leader" >&2; exit 1; }
docker run -d --name "$PG2" --network "$NET" \
    -e SCOPE=aip-cut -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg2 -e POD_IP="$PG2" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
for _ in $(seq 1 90); do
    role2=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role2" = "replica" ] && break
    sleep 2
done
[ "$role2" = "replica" ] || { echo "no replica" >&2; exit 1; }
docker exec "$PG1" psql -q -U postgres -d postgres -c "alter user postgres password 'postgres'"

cat > /tmp/cut-hap.cfg <<CFG
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
docker run -d --name "$HAP" --network "$NET" -v /tmp/cut-hap.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro haproxy:3.0-alpine >/dev/null
sleep 3

echo "dumping main database"
docker compose exec -T postgres pg_dump -U aip --no-owner --no-privileges aip | sed -e '/transaction_timeout/d' -e '/^\\restrict/d' -e '/^\\unrestrict/d' > "$DUMP"
echo "dump_bytes=$(stat -c %s "$DUMP")"
docker cp "$DUMP" "$PG1:/tmp/aip.sql" >/dev/null
docker exec "$PG1" psql -q -U postgres -d postgres -c "create database aip" >/dev/null
docker exec "$PG1" psql -q -U postgres -d aip -v ON_ERROR_STOP=1 -f /tmp/aip.sql >/dev/null
docker exec -i "$PG1" psql -q -U postgres -d aip < "$ROOT/deploy/postgres/00_roles.sql" >/dev/null
echo "restored"

echo "building aipctl/aipd"
(cd "$ROOT/apps/api" && CGO_ENABLED=0 go build -o /tmp/aipctl ./cmd/aipctl && CGO_ENABLED=0 go build -o /tmp/aipd ./cmd/aipd)
docker run --rm --network "$NET" -v /tmp/aipctl:/aipctl:ro \
    -e AIP_DB_MIGRATOR_DSN="postgres://postgres:postgres@$HAP:5432/aip?sslmode=require" \
    alpine:3.20 /aipctl migrate up >/dev/null

docker exec "$PG1" psql -q -U postgres -d aip -c "grant select,insert,update,delete on all tables in schema public to aip_app; grant usage,select on all sequences in schema public to aip_app" >/dev/null
counts=$(docker exec "$PG1" psql -U postgres -d aip -tAc "select (select count(*) from orgs) || '/' || (select count(*) from projects) || '/' || (select count(*) from tasks) || '/' || (select count(*) from runs)")
echo "counts_orgs/projects/tasks/runs=$counts"

docker run -d --name "$API" --network "$NET" -v /tmp/aipd:/aipd:ro -v "$ROOT/jwt.pem:/jwt.pem:ro" \
    -e AIP_DB_DSN="postgres://aip_app:app@$HAP:5432/aip?sslmode=require" \
    -e AIP_JWT_KEY=/jwt.pem -e AIP_HTTP_ADDR=":8080" -p 18084:8080 docker:27-cli /aipd >/dev/null
for _ in $(seq 1 30); do
    code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:18084/readyz || true)
    [ "$code" = "200" ] && break
    sleep 1
done
echo "readyz=$code"
login=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://localhost:18084/api/v1/auth/login \
    -H 'Content-Type: application/json' -d '{"email":"demo@acme.io","password":"correct horse battery"}')
echo "login=$login"
echo "cutover rehearsal ok"
