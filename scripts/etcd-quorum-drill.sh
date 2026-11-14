#!/usr/bin/env bash
# Isolated 3-node etcd quorum drill.
set -euo pipefail
SUFFIX="$$"
NET="aip-etcd-quorum-${SUFFIX}"
N1="aip-etcd1-${SUFFIX}"; N2="aip-etcd2-${SUFFIX}"; N3="aip-etcd3-${SUFFIX}"
cleanup() {
    docker rm -f "$N1" "$N2" "$N3" >/dev/null 2>&1 || true
    docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$NET" >/dev/null
cluster="etcd1=http://$N1:2380,etcd2=http://$N2:2380,etcd3=http://$N3:2380"
for pair in "1 $N1" "2 $N2" "3 $N3"; do
    set -- $pair
    name="etcd$1"
    node="$2"
    docker run -d --name "$node" --network "$NET" quay.io/coreos/etcd:v3.5.15 \
        /usr/local/bin/etcd --name "$name" --initial-cluster-token aip-quorum \
        --initial-advertise-peer-urls "http://$node:2380" --listen-peer-urls http://0.0.0.0:2380 \
        --advertise-client-urls "http://$node:2379" --listen-client-urls http://0.0.0.0:2379 \
        --initial-cluster "$cluster" --initial-cluster-state new >/dev/null
done
sleep 6

health() {
    docker run --rm --network "$NET" quay.io/coreos/etcd:v3.5.15 \
        /usr/local/bin/etcdctl --endpoints="http://$N1:2379,http://$N2:2379,http://$N3:2379" \
        endpoint health --cluster 2>/dev/null | grep -c "is healthy" || true
}
echo "healthy_before=$(health)"

docker stop "$N3" >/dev/null
sleep 2
echo "healthy_after_one_down=$(health)"

docker stop "$N2" >/dev/null
sleep 2
healthy=$(health)
echo "healthy_after_two_down=$healthy"
[ "$healthy" -eq 0 ] || { echo "expected quorum loss" >&2; exit 1; }

docker start "$N2" "$N3" >/dev/null
sleep 4
echo "healthy_after_restart=$(health)"
echo "etcd quorum drill ok"
