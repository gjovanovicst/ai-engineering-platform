#!/usr/bin/env bash
# Load/soak check: boots a stack, starts N planning runs, waits for the queue to
# drain and reports throughput, queue wait and run duration. Fails on failed
# runs, leaked sandboxes or a p95 above LOAD_P95_LIMIT seconds.
#
#   LOAD_N=20 make loadtest
#   KEEP=1 LOAD_N=5 make loadtest   # keep the stack for inspection
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT=$PWD

N=${LOAD_N:-10}
P95_LIMIT=${LOAD_P95_LIMIT:-120}
API=aip-load-api
WORKER=aip-load-worker
NETWORK=aip_default
IMAGE=${AIP_SANDBOX_IMAGE:-aip/sandbox:go1.25-node22}

log() { printf '\n[load] %s\n' "$1"; }
fail() { printf '\n[load] FAILED: %s\n' "$1" >&2; exit 1; }
cleanup() {
  docker rm -f "$API" "$WORKER" >/dev/null 2>&1 || true
  [ "${KEEP:-0}" = "1" ] || docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "build"
(cd apps/api && CGO_ENABLED=0 go build -o /tmp/aipd ./cmd/aipd \
  && CGO_ENABLED=0 go build -o /tmp/aip-worker ./cmd/aip-worker \
  && CGO_ENABLED=0 go build -o /tmp/aipctl ./cmd/aipctl)

log "stack"
cleanup
docker compose up -d postgres >/dev/null
for _ in $(seq 1 30); do
  docker compose exec -T postgres pg_isready -U aip >/dev/null 2>&1 && break
  sleep 1
done
docker run --rm --network "$NETWORK" -v /tmp/aipctl:/aipctl:ro \
  -e AIP_DB_MIGRATOR_DSN="postgres://aip_migrator:migrator@postgres:5432/aip?sslmode=disable" \
  alpine:3.20 /aipctl migrate up >/dev/null
docker run -d --rm --name "$API" --network "$NETWORK" \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_JWT_KEY=/jwt.pem -e AIP_LOG_LEVEL=warn \
  -v /tmp/aipd:/aipd:ro -v "$ROOT/jwt.pem:/jwt.pem:ro" docker:27-cli /aipd >/dev/null
docker run -d --rm --name "$WORKER" --network "$NETWORK" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_SANDBOX_IMAGE="$IMAGE" -e AIP_TEST_COMMAND=true -e AIP_LOG_LEVEL=warn \
  -e "AIP_AGENT_COMMAND=mkdir -p src; echo load > src/x.txt; echo 'AIP_REVIEW: {\"findings\":[]}'" -v /tmp/aip-worker:/aip-worker:ro \
  docker:27-cli /aip-worker >/dev/null

C="docker run --rm --network $NETWORK alpine:3.20"
api() { $C sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' $1"; }
for _ in $(seq 1 30); do
  $C sh -c "wget -qO- http://$API:8080/healthz" >/dev/null 2>&1 && break
  sleep 1
done

log "seed $N tasks"
EMAIL="load-$(date -u +%s)@acme.io"
$C sh -c "wget -qO- --header='Content-Type: application/json' --post-data='{\"email\":\"$EMAIL\",\"name\":\"Load\",\"password\":\"correct horse battery\",\"org_name\":\"Load\"}' http://$API:8080/api/v1/auth/register" >/dev/null
TOKEN=$($C sh -c "wget -qO- --header='Content-Type: application/json' --post-data='{\"email\":\"$EMAIL\",\"password\":\"correct horse battery\"}' http://$API:8080/api/v1/auth/login" | sed 's/.*"access_token":"\([^"]*\)".*/\1/')
ORG=$($C sh -c "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API:8080/api/v1/auth/me" | sed 's/.*"org_id":"\([^"]*\)".*/\1/')
PROJECT=$($C sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"name\":\"Load\"}' http://$API:8080/api/v1/orgs/$ORG/projects" | sed 's/.*"id":"\([^"]*\)".*/\1/')

START=$(date +%s)
for i in $(seq 1 "$N"); do
  TASK=$($C sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"title\":\"Load task $i\",\"acceptance\":[{\"given\":\"g\",\"when\":\"w\",\"then\":\"t\"}]}' http://$API:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks")
  REF=$(echo "$TASK" | sed 's/.*"ref":"\([^"]*\)".*/\1/')
  $C sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"to\":\"ready\"}' http://$API:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF/transitions" >/dev/null
  $C sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"agent_type\":\"development\"}' http://$API:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF/runs" >/dev/null
done

log "drain queue"
DEADLINE=$((START + N * 60 + 120))
while :; do
  PENDING=$(docker compose exec -T postgres psql -U aip -d aip -tAc "select count(*) from runs where status in ('queued','running')" | tr -d ' ')
  [ "$PENDING" = "0" ] && break
  [ "$(date +%s)" -gt "$DEADLINE" ] && fail "queue did not drain (pending=$PENDING)"
  sleep 2
done
ELAPSED=$(( $(date +%s) - START ))

log "report"
docker compose exec -T postgres psql -U aip -d aip -F'|' -tAc "
select
  count(*) filter (where status='succeeded'),
  count(*) filter (where status='failed'),
  round(percentile_cont(0.5) within group (order by extract(epoch from (started_at-created_at)))::numeric, 1),
  round(percentile_cont(0.95) within group (order by extract(epoch from (started_at-created_at)))::numeric, 1),
  round(percentile_cont(0.95) within group (order by extract(epoch from (finished_at-started_at)))::numeric, 1)
from runs" | while IFS='|' read -r OK FAILED Q50 Q95 R95; do
  printf '  succeeded=%s failed=%s queue_wait_p50=%ss p95=%ss run_p95=%ss\n' "$OK" "$FAILED" "$Q50" "$Q95" "$R95"
  [ "$FAILED" = "0" ] || fail "$FAILED run(s) failed"
  awk -v v="$R95" -v l="$P95_LIMIT" 'BEGIN{exit !(v<=l)}' || fail "run p95 ${R95}s > ${P95_LIMIT}s"
done
printf '  throughput=%s runs in %ss (%.1f/s)\n' "$N" "$ELAPSED" "$(awk -v n="$N" -v e="$ELAPSED" 'BEGIN{print (e>0? n/e : n)}')"

LEAKS=$(docker ps -q --filter name=aip-sbx- | wc -l | tr -d ' ')
printf '  sandbox_containers=%s\n' "$LEAKS"
[ "$LEAKS" = "0" ] || fail "$LEAKS sandbox container(s) leaked"

log "OK — $N runs drained in ${ELAPSED}s, no leaks"
