#!/usr/bin/env bash
# Recreates the API and worker with the env from ~/.aip-demo.env and verifies the
# GitHub token before starting anything. Usage: ./scripts/restart-demo.sh
set -euo pipefail
cd "$(dirname "$0")/.."

ENV_FILE="${AIP_ENV_FILE:-$HOME/.aip-demo.env}"
if [ ! -f "$ENV_FILE" ]; then
  echo "missing $ENV_FILE (copy the template from docs/22-deepseek-github.md)" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${AIP_GITHUB_TOKEN:?AIP_GITHUB_TOKEN is empty}"
: "${AIP_SECRETS_KEY:?AIP_SECRETS_KEY is empty}"

code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $AIP_GITHUB_TOKEN" https://api.github.com/user)
if [ "$code" != "200" ]; then
  echo "GitHub token is not valid (GET /user -> $code). Create a new fine-grained PAT." >&2
  exit 1
fi
echo "GitHub token: valid"

echo "building binaries"
export GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/gomod" GOPATH="$PWD/.cache/gopath"
(cd apps/api && CGO_ENABLED=0 go build -o /tmp/aipd ./cmd/aipd \
  && CGO_ENABLED=0 go build -o /tmp/aip-worker ./cmd/aip-worker)

docker rm -f aip-demo-api aip-demo-worker >/dev/null 2>&1 || true

docker run -d --name aip-demo-api --network aip_default -p 18080:8080 \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_JWT_KEY=/jwt.pem -e AIP_WEB_URL=http://localhost:3000 \
  -e AIP_SMTP_ADDR=mailhog:1025 \
  -e AIP_SECRETS_KEY -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET \
  -e AIP_GITHUB_CLONE_BASE -e AIP_GITHUB_API_URL \
  -e AIP_PRICE_INPUT_PER_1K -e AIP_PRICE_OUTPUT_PER_1K \
  -v "$PWD/jwt.pem:/jwt.pem:ro" -v /tmp/aipd:/aipd:ro docker:27-cli /aipd >/dev/null

docker run -d --name aip-demo-worker --network aip_default \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_SECRETS_KEY -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET \
  -e AIP_GITHUB_CLONE_BASE -e AIP_GITHUB_API_URL \
  -e AIP_AGENT_BACKEND -e AIP_LLM_MAX_ATTEMPTS -e AIP_TEST_COMMAND \
  -e AIP_PRICE_INPUT_PER_1K -e AIP_PRICE_OUTPUT_PER_1K \
  -e DOCKER_HOST=unix:///var/run/docker.sock \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /tmp/aip-worker:/aip-worker:ro docker:27-cli /aip-worker >/dev/null

sleep 3
code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:18080/healthz)
echo "api health: $code"
docker logs aip-demo-worker 2>&1 | tail -2
