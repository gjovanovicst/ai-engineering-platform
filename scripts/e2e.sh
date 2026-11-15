#!/usr/bin/env bash
# End-to-end: task -> plan -> approval -> development (diff+tests) -> push ->
# MR -> PR approval gate -> merge -> done -> metrics. Uses a fake GitLab API and
# a local bare repo, so it runs without any external account.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT=$PWD
export GOCACHE=${GOCACHE:-$ROOT/.cache/go-build}
export GOMODCACHE=${GOMODCACHE:-$ROOT/.cache/gomod}
export GOPATH=${GOPATH:-$ROOT/.cache/gopath}
API_CONTAINER=aip-e2e-api
WORKER_CONTAINER=aip-e2e-worker
GITLAB_CONTAINER=aip-e2e-gitlab
PG_CONTAINER=aip-e2e-postgres
NETWORK=aip_default
IMAGE=aip/sandbox:go1.25-node22
REPOS=/tmp/aip-e2e-repos
CREATED_NETWORK=0

log() { printf '\n[e2e] %s\n' "$1"; }
fail() { printf '\n[e2e] FAILED: %s\n' "$1" >&2; exit 1; }
cleanup() {
  if [ "${KEEP:-0}" = "1" ]; then
    echo "[e2e] keeping containers (KEEP=1)"
    return
  fi
  docker rm -f "$API_CONTAINER" "$WORKER_CONTAINER" "$GITLAB_CONTAINER" "$PG_CONTAINER" >/dev/null 2>&1 || true
  if [ "$CREATED_NETWORK" = "1" ]; then
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

log "build binaries"
(cd "$ROOT/apps/api" && CGO_ENABLED=0 go build -o /tmp/aipd ./cmd/aipd \
  && CGO_ENABLED=0 go build -o /tmp/aip-worker ./cmd/aip-worker \
  && CGO_ENABLED=0 go build -o /tmp/aipctl ./cmd/aipctl)

log "start dependencies"
cleanup
if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
  docker network create "$NETWORK" >/dev/null
  CREATED_NETWORK=1
fi
docker run -d --name "$PG_CONTAINER" --network "$NETWORK" \
  -e POSTGRES_USER=aip -e POSTGRES_PASSWORD=aip -e POSTGRES_DB=aip \
  -v "$ROOT/deploy/postgres/00_roles.sql:/docker-entrypoint-initdb.d/00_roles.sql:ro" \
  postgres:17-alpine >/dev/null
for _ in $(seq 1 30); do
  docker exec "$PG_CONTAINER" pg_isready -U aip >/dev/null 2>&1 && break
  sleep 1
done
docker run --rm -v /tmp:/tmp alpine:3.20 rm -rf /tmp/aip-e2e-repos >/dev/null 2>&1 || true
rm -rf "$REPOS" 2>/dev/null || true
mkdir -p "$REPOS/group"; git init --bare -q "$REPOS/group/project.git"
docker run --rm -v "$REPOS:/repos" alpine:3.20 chown -R 0:0 /repos
docker run -d --rm --name "$GITLAB_CONTAINER" --network "$NETWORK" \
  -v "$ROOT/scripts/fakegitlab.py:/fakegitlab.py:ro" python:3-alpine python /fakegitlab.py >/dev/null

docker run --rm --network "$NETWORK" -v /tmp/aipctl:/aipctl:ro \
  -e AIP_DB_MIGRATOR_DSN="postgres://aip_migrator:migrator@aip-e2e-postgres:5432/aip?sslmode=disable" \
  alpine:3.20 /aipctl migrate up >/dev/null

log "start api and worker"
docker run -d --rm --name "$API_CONTAINER" --network "$NETWORK" \
  -e AIP_DB_DSN="postgres://aip_app:app@aip-e2e-postgres:5432/aip?sslmode=disable" \
  -e AIP_JWT_KEY=/jwt.pem -e AIP_LOG_LEVEL=warn \
  -v /tmp/aipd:/aipd:ro -v "$ROOT/jwt.pem:/jwt.pem:ro" docker:27-cli /aipd >/dev/null

AGENT_CMD="if grep -q 'PLANNING agent' /workspace/.aip/prompt.txt; then echo 'AIP_PLAN: {\"steps\":[\"write file\"],\"files\":[\"src/x.txt\"],\"risks\":[]}'; elif grep -q 'REVIEW agent' /workspace/.aip/prompt.txt; then echo 'AIP_REVIEW: {\"findings\":[]}'; else mkdir -p src; echo hi > src/x.txt; echo 'AIP_PROOF: {\"evidence\":[{\"criterion\":\"AC1\",\"kind\":\"manual\",\"reason\":\"e2e verification\"}]}'; fi"
docker run -d --rm --name "$WORKER_CONTAINER" --network "$NETWORK" \
  -v /var/run/docker.sock:/var/run/docker.sock -v "$REPOS:/repos" \
  -e AIP_DB_DSN="postgres://aip_app:app@aip-e2e-postgres:5432/aip?sslmode=disable" \
  -e AIP_SANDBOX_IMAGE="$IMAGE" -e AIP_TEST_COMMAND="true" -e AIP_LOG_LEVEL=warn \
  -e "AIP_AGENT_COMMAND=$AGENT_CMD" -e GITLAB_TOKEN=gl-token \
  -e AIP_GITLAB_API_URL=http://"$GITLAB_CONTAINER":5000 -e AIP_GITLAB_CLONE_BASE=file:///repos \
  -v /tmp/aip-worker:/aip-worker:ro docker:27-cli /aip-worker >/dev/null

curl_json() { docker run --rm --network "$NETWORK" alpine:3.20 sh -c "$1"; }
for _ in $(seq 1 30); do
  curl_json "wget -qO- http://$API_CONTAINER:8080/healthz" >/dev/null 2>&1 && break
  sleep 1
done

log "register and login"
curl_json "wget -qO- --header='Content-Type: application/json' --post-data='{\"email\":\"e2e@acme.io\",\"name\":\"E2E\",\"password\":\"correct horse battery\",\"org_name\":\"Acme\"}' http://$API_CONTAINER:8080/api/v1/auth/register" >/dev/null
TOKEN=$(curl_json "wget -qO- --header='Content-Type: application/json' --post-data='{\"email\":\"e2e@acme.io\",\"password\":\"correct horse battery\"}' http://$API_CONTAINER:8080/api/v1/auth/login" | sed 's/.*"access_token":"\([^"]*\)".*/\1/')
ORG=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/auth/me" | sed 's/.*"org_id":"\([^"]*\)".*/\1/')
PROJECT=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"name\":\"E2E\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects" | sed 's/.*"id":"\([^"]*\)".*/\1/')
log "select custom agent plugin"
PLUGIN_IMAGE=$(docker inspect --format '{{index .RepoDigests 0}}' "$IMAGE")
[ -n "$PLUGIN_IMAGE" ] || fail "sandbox image has no repo digest"
python3 - "$AGENT_CMD" "$PLUGIN_IMAGE" > /tmp/aip-e2e-plugin.json <<'PYJSON'
import json, sys
print(json.dumps({"name": "e2e-agent", "version": "1.0.0", "image": sys.argv[2],
                  "command": sys.argv[1],
                  "agent_types": ["planning", "development", "review"]}))
PYJSON
PLUGIN=$(docker run --rm --network "$NETWORK" -v /tmp/aip-e2e-plugin.json:/plugin.json alpine:3.20 sh -c "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-file=/plugin.json http://$API_CONTAINER:8080/api/v1/orgs/$ORG/agent-plugins")
PLUGIN_ID=$(echo "$PLUGIN" | sed 's/.*"id":"\([^"]*\)".*/\1/')
echo "$PLUGIN_ID" | grep -Eq '^[0-9a-f-]{36}$' || fail "plugin was not registered: $PLUGIN"
docker run --rm --network "$NETWORK" curlimages/curl:8.10.1 -s -X PUT -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"plugin_id\":\"$PLUGIN_ID\"}" http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/agent-plugin >/dev/null
TASK=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"title\":\"Add SSO\",\"acceptance\":[{\"given\":\"g\",\"when\":\"w\",\"then\":\"t\"}]}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks")
REF=$(echo "$TASK" | sed 's/.*"ref":"\([^"]*\)".*/\1/')
REPO=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"provider\":\"gitlab\",\"name\":\"group/project\",\"webhook_secret\":\"hook-secret\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/repos" | sed 's/.*"id":"\([^"]*\)".*/\1/')
if [ "${E2E_CONCURRENT:-0}" = "1" ]; then
  log "concurrent PRs (${N:-10})"
  docker run --rm --network "$NETWORK" -v "$ROOT/scripts/e2e_concurrent.py:/e2e.py:ro"     -e API_BASE="http://$API_CONTAINER:8080" -e ORG="$ORG" -e PROJECT="$PROJECT"     -e REPO="$REPO" -e TOKEN="$TOKEN" -e N="${N:-10}" -e WAIT_PR="${WAIT_PR:-900}" python:3-alpine python /e2e.py
  exit 0
fi

log "plan"
READY=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"to\":\"ready\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF/transitions")
echo "$READY" | grep -q '"status":"ready"' || fail "task did not become ready: $READY"
STARTED=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"agent_type\":\"planning\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF/runs")
echo "$STARTED" | grep -q '"id"' || fail "run was not started: $STARTED"
STATUS=""
for _ in $(seq 1 30); do
  STATUS=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF/runs" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p' | head -1)
  [ "$STATUS" = "awaiting_approval" ] && break
  sleep 1
done
[ "$STATUS" = "awaiting_approval" ] || fail "planning run did not await approval (last status='$STATUS')"

APPROVAL=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/approvals" | sed 's/.*"id":"\([^"]*\)".*/\1/')
curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"decision\":\"approved\",\"reason\":\"ok\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/approvals/$APPROVAL/decision" >/dev/null

log "development, push and MR"
for _ in $(seq 1 60); do
  BRANCHES=$(git --git-dir "$REPOS/group/project.git" branch -a 2>/dev/null || true)
  PRS=$(docker exec "$PG_CONTAINER" psql -U aip -d aip -tAc "select count(*) from pull_requests" | tr -d ' ')
  [ "$PRS" = "1" ] && [ -n "$BRANCHES" ] && break
  sleep 1
done
[ "$PRS" = "1" ] || fail "no pull request was recorded"
git --git-dir "$REPOS/group/project.git" branch -a | grep -q 'aip/' || fail "no aip branch was pushed"

PR_NUM=$(docker exec "$PG_CONTAINER" psql -U aip -d aip -tAc "select number from pull_requests limit 1" | tr -d ' ')
[ -n "$PR_NUM" ] || fail "no pull request number in the database"
log "merge before approval must be blocked (PR #$PR_NUM)"
BODY="{\"object_kind\":\"merge_request\",\"project\":{\"path_with_namespace\":\"group/project\"},\"object_attributes\":{\"iid\":$PR_NUM,\"action\":\"merge\",\"state\":\"merged\"}}"
OUT=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='X-Gitlab-Token: hook-secret' --post-data='$BODY' http://$API_CONTAINER:8080/webhooks/gitlab/$REPO")
echo "$OUT" | grep -q 'approval_required' || fail "merge was not gated: $OUT"

log "approve the PR and merge"
PR_APPROVAL=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/approvals" | sed 's/.*"id":"\([^"]*\)".*/\1/')
curl_json "wget -qO- --header='Content-Type: application/json' --header='Authorization: Bearer $TOKEN' --post-data='{\"decision\":\"approved\",\"reason\":\"ship\"}' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/approvals/$PR_APPROVAL/decision" >/dev/null
OUT=$(curl_json "wget -qO- --header='Content-Type: application/json' --header='X-Gitlab-Token: hook-secret' --post-data='$BODY' http://$API_CONTAINER:8080/webhooks/gitlab/$REPO")
echo "$OUT" | grep -q 'merged' || fail "merge after approval failed: $OUT"
TASK_STATUS=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/projects/$PROJECT/tasks/$REF" | sed 's/.*"status":"\([^"]*\)".*/\1/')
[ "$TASK_STATUS" = "done" ] || fail "task is not done (status=$TASK_STATUS)"

log "metrics"
METRICS=$(curl_json "wget -qO- --header='Authorization: Bearer $TOKEN' http://$API_CONTAINER:8080/api/v1/orgs/$ORG/metrics?days=30")
echo "$METRICS" | grep -q '"succeeded_runs":3' || fail "unexpected metrics: $METRICS"

printf '\n[e2e] OK — plan, approval, diff, MR, gate, merge, done, metrics\n'
