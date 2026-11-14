#!/usr/bin/env bash
# Cold volume backup of the Patroni replica plus archive validation.
set -euo pipefail
REPLICA=${REPLICA:-aip-patroni-patroni2-1}
VOLUME=${VOLUME:-aip-patroni_patroni2-data}
TEST_VOLUME=${TEST_VOLUME:-aip-patroni-restore-test}
ARCHIVE=/tmp/patroni2-data.tgz
API=${API:-http://localhost:18080/readyz}

docker stop "$REPLICA" >/dev/null
docker run --rm -v "$VOLUME":/data:ro -v /tmp:/backup alpine:3.20 tar czf "/backup/$(basename "$ARCHIVE")" -C /data .
echo "archive_bytes=$(stat -c %s "$ARCHIVE")"
code=$(curl -s -o /dev/null -m 5 -w '%{http_code}' "$API")
echo "api_during_stop=$code"
[ "$code" = "200" ] || { docker start "$REPLICA" >/dev/null; echo "api failed during stop" >&2; exit 1; }
docker start "$REPLICA" >/dev/null

docker volume create "$TEST_VOLUME" >/dev/null
docker run --rm -v "$TEST_VOLUME":/data -v /tmp:/backup alpine:3.20 tar xzf "/backup/$(basename "$ARCHIVE")" -C /data
docker run --rm -v "$TEST_VOLUME":/data postgres:16-alpine sh -c 'pg_controldata /data/pgroot/data | head -5'
docker volume rm "$TEST_VOLUME" >/dev/null
docker run --rm -v /tmp:/tmp alpine:3.20 rm -f "$ARCHIVE"
echo "patroni volume backup drill ok"
