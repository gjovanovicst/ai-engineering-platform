#!/usr/bin/env bash
# Stop one Patroni replica, verify the platform stays up, restart and wait for
# streaming replication to catch up.
set -euo pipefail
PRIMARY=${PRIMARY:-aip-patroni-patroni1-1}
REPLICA=${REPLICA:-aip-patroni-patroni2-1}
API=${API:-http://localhost:18080/readyz}

code=$(curl -s -o /dev/null -m 5 -w '%{http_code}' "$API")
[ "$code" = "200" ] || { echo "api not ready before drill: $code" >&2; exit 1; }
docker stop "$REPLICA" >/dev/null
sleep 5
code=$(curl -s -o /dev/null -m 5 -w '%{http_code}' "$API")
rows=$(docker exec "$PRIMARY" psql -U postgres -d aip -tAc "select count(*) from orgs")
echo "during_stop api=$code orgs=$rows"
[ "$code" = "200" ] || { echo "api failed while replica was down" >&2; exit 1; }

docker start "$REPLICA" >/dev/null
state=""; lag="-1"
for _ in $(seq 1 60); do
    state=$(docker exec "$REPLICA" psql -U postgres -d postgres -tAc "select pg_is_in_recovery()" 2>/dev/null || true)
    lag=$(docker exec "$PRIMARY" psql -U postgres -d aip -tAc "select coalesce(max(extract(epoch from replay_lag)),-1) from pg_stat_replication" 2>/dev/null || echo -1)
    [ "$state" = "t" ] && [ "$lag" != "-1" ] && break
    sleep 2
done
echo "rejoined recovery=$state replay_lag=$lag"
[ "$state" = "t" ] || { echo "replica did not rejoin" >&2; exit 1; }
[ "$lag" != "-1" ] || { echo "replication did not resume" >&2; exit 1; }
echo "patroni replica drill ok"
