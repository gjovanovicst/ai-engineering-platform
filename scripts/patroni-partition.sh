#!/usr/bin/env bash
# Isolated network partition: leader loses etcd but keeps the client network.
set -euo pipefail
SUFFIX="$$"
DCS="aip-dcs-${SUFFIX}"; CLIENT="aip-client-${SUFFIX}"
ETCD="aip-petcd-${SUFFIX}"; PG1="aip-pp1-${SUFFIX}"; PG2="aip-pp2-${SUFFIX}"
cleanup() {
    docker rm -f "$PG1" "$PG2" "$ETCD" >/dev/null 2>&1 || true
    docker network rm "$DCS" "$CLIENT" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$DCS" >/dev/null
docker network create "$CLIENT" >/dev/null
docker run -d --name "$ETCD" --network "$DCS" quay.io/coreos/etcd:v3.5.15 \
    /usr/local/bin/etcd --name etcd --enable-v2=true \
    --advertise-client-urls "http://$ETCD:2379" --listen-client-urls http://0.0.0.0:2379 \
    --initial-advertise-peer-urls "http://$ETCD:2380" --listen-peer-urls http://0.0.0.0:2380 \
    --initial-cluster "etcd=http://$ETCD:2380" --initial-cluster-state new >/dev/null
sleep 4

docker run -d --name "$PG1" --network "$DCS" \
    -e SCOPE=aip-part -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg1 -e POD_IP="$PG1" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
docker network connect "$CLIENT" "$PG1"
for _ in $(seq 1 60); do
    role=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role" = "master" ] && break
    sleep 2
done
[ "$role" = "master" ] || { echo "no leader" >&2; exit 1; }
echo "leader=pg1"

docker run -d --name "$PG2" --network "$DCS" \
    -e SCOPE=aip-part -e PGVERSION=16 -e ETCD_HOSTS="$ETCD:2379" \
    -e PATRONI_NAME=pg2 -e POD_IP="$PG2" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
docker network connect "$CLIENT" "$PG2"
for _ in $(seq 1 90); do
    role2=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role2" = "replica" ] && break
    sleep 2
done
[ "$role2" = "replica" ] || { echo "no replica" >&2; exit 1; }
echo "replica=pg2"

docker network disconnect "$DCS" "$PG1"
echo "partition=pg1-from-etcd"
for _ in $(seq 1 45); do sleep 1; done

role=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
role2=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
if docker exec "$PG1" psql -q -U postgres -d postgres -c "create table partition_probe(x int)" >/dev/null 2>&1; then
    write="accepted"
else
    write="rejected"
fi
p1=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s -o /dev/null -w '%{http_code}' "http://$PG1:8008/primary" 2>/dev/null || true)
r1=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s -o /dev/null -w '%{http_code}' "http://$PG1:8008/replica" 2>/dev/null || true)
p2=$(docker run --rm --network "$CLIENT" curlimages/curl:8.10.1 -s -o /dev/null -w '%{http_code}' "http://$PG2:8008/primary" 2>/dev/null || true)
echo "partitioned_role=$role other_role=$role2 partitioned_write=$write pg1_primary=$p1 pg1_replica=$r1 pg2_primary=$p2"
[ "$write" = "rejected" ] || { echo "former leader still accepted writes" >&2; exit 1; }
echo "patroni partition drill ok"
