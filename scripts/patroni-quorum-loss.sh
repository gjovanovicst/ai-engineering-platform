#!/usr/bin/env bash
# Isolated: what does Patroni do when etcd loses quorum while a leader runs?
set -euo pipefail
SUFFIX="$$"
NET="aip-pql-${SUFFIX}"
E1="aip-e1-${SUFFIX}"; E2="aip-e2-${SUFFIX}"; E3="aip-e3-${SUFFIX}"
PG1="aip-pq1-${SUFFIX}"; PG2="aip-pq2-${SUFFIX}"
cleanup() {
    if [ "${KEEP:-0}" = "1" ]; then
        echo "kept containers for debugging"
        return
    fi
    docker rm -f "$PG1" "$PG2" "$E1" "$E2" "$E3" >/dev/null 2>&1 || true
    docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
cluster="etcd1=http://$E1:2380,etcd2=http://$E2:2380,etcd3=http://$E3:2380"
for pair in "1 $E1" "2 $E2" "3 $E3"; do
    set -- $pair
    docker run -d --name "$2" --network "$NET" quay.io/coreos/etcd:v3.5.15 \
        /usr/local/bin/etcd --name "etcd$1" --enable-v2=true --initial-cluster-token aip-pql \
        --initial-advertise-peer-urls "http://$2:2380" --listen-peer-urls http://0.0.0.0:2380 \
        --advertise-client-urls "http://$2:2379" --listen-client-urls http://0.0.0.0:2379 \
        --initial-cluster "$cluster" --initial-cluster-state new >/dev/null
done
sleep 5
hosts="$E1:2379,$E2:2379,$E3:2379"

docker run -d --name "$PG1" --network "$NET" \
    -e SCOPE=aip-pql -e PGVERSION=16 -e ETCD_HOSTS="$hosts" \
    -e PATRONI_NAME=pg1 -e POD_IP="$PG1" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
for _ in $(seq 1 90); do
    role=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role" = "master" ] && break
    sleep 2
done
[ "$role" = "master" ] || { echo "no leader" >&2; exit 1; }
echo "leader=pg1"

docker run -d --name "$PG2" --network "$NET" \
    -e SCOPE=aip-pql -e PGVERSION=16 -e ETCD_HOSTS="$hosts" \
    -e PATRONI_NAME=pg2 -e POD_IP="$PG2" -e PGPORT=5432 \
    ghcr.io/zalando/spilo-16:3.3-p1 >/dev/null
for _ in $(seq 1 90); do
    role2=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG2:8008/patroni" 2>/dev/null \
        | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
    [ "$role2" = "replica" ] && break
    sleep 2
done
[ "$role2" = "replica" ] || { echo "no replica" >&2; exit 1; }
echo "replica=pg2"

docker stop "$E2" "$E3" >/dev/null
echo "etcd_quorum=lost"
for _ in $(seq 1 45); do sleep 1; done

role=$(docker run --rm --network "$NET" curlimages/curl:8.10.1 -s "http://$PG1:8008/patroni" 2>/dev/null \
    | python3 -c "import sys,json;print(json.load(sys.stdin).get('role'))" 2>/dev/null || true)
echo "leader_role_after_45s=$role"
if docker exec "$PG1" psql -q -U postgres -d postgres -c "create table quorum_write_probe(x int)" >/dev/null 2>&1; then
    echo "writes_after_quorum_loss=accepted"
else
    echo "writes_after_quorum_loss=rejected"
fi
echo "patroni quorum-loss drill done"
