#!/usr/bin/env bash
# Restarts the web dev server cleanly. Stale `next dev`/`next build` processes
# sharing .next produce hydration mismatches and missing CSS; this script always
# starts from a single process.
#
#   ./scripts/dev-web.sh --clean     # also remove .next
set -euo pipefail
cd "$(dirname "$0")/../apps/web"

PORT="${PORT:-3000}"
API="${AIP_WEB_API_URL:-http://localhost:18080}"

# lsof does not always see the socket (namespaces, other shells); try ss and
# fuser as well, then verify that the port is actually free.
port_pids() {
  local pids=""
  pids=$(lsof -ti:"$PORT" 2>/dev/null || true)
  [ -z "$pids" ] && pids=$(fuser "$PORT/tcp" 2>/dev/null || true)
  [ -z "$pids" ] && pids=$(ss -ltnp 2>/dev/null | awk -v p=":$PORT" \
    '$4 ~ p { if (match($0, /pid=[0-9]+/)) print substr($0, RSTART+4, RLENGTH-4) }')
  echo "$pids"
}

if pids=$(port_pids); [ -n "$pids" ]; then
  pids=$(echo $pids) # fuser pads with spaces
  echo "stopping process(es) on :$PORT: $pids"
  # shellcheck disable=SC2086
  kill $pids 2>/dev/null || true
  sleep 2
fi

if ss -ltn 2>/dev/null | grep -q ":$PORT "; then
  pids=$(port_pids)
  # shellcheck disable=SC2086
  [ -n "$pids" ] && kill -9 $pids 2>/dev/null || true
  sleep 1
fi

if ss -ltn 2>/dev/null | grep -q ":$PORT "; then
  echo "port $PORT is still in use; free it with: fuser -k $PORT/tcp" >&2
  exit 1
fi

if [ "${1:-}" = "--clean" ]; then
  echo "removing .next"
  rm -rf .next
fi

echo "starting web on http://localhost:$PORT (API $API)"
exec env NEXT_PUBLIC_API_URL="$API" NEXT_TELEMETRY_DISABLED=1 ./node_modules/.bin/next dev -p "$PORT"
