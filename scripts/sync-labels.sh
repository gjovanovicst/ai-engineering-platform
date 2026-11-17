#!/usr/bin/env bash
# Creates or updates the labels the backlog filters on (docs/13-setup.md §8).
#
#   AIP_GITHUB_TOKEN=... scripts/sync-labels.sh [owner/repo] [--dry-run]
#
# The token needs Issues: write on the repository. The owner/repo defaults to
# the origin remote. Existing labels are updated in place, so the script is
# idempotent.
set -euo pipefail

dry_run=0
repo=""
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry_run=1 ;;
    *) repo="$arg" ;;
  esac
done

if [ -z "$repo" ]; then
  remote=$(git remote get-url origin)
  case "$remote" in
    git@github.com:*) repo=${remote#git@github.com:}; repo=${repo%.git} ;;
    https://github.com/*) repo=${remote#https://github.com/}; repo=${repo%.git} ;;
    *) echo "sync-labels: pass owner/repo (cannot parse $remote)" >&2; exit 1 ;;
  esac
fi

token=${AIP_GITHUB_TOKEN:-${GITHUB_TOKEN:-}}
if [ -z "$token" ] && [ "$dry_run" -eq 0 ]; then
  echo "sync-labels: set AIP_GITHUB_TOKEN (needs Issues: write)" >&2
  exit 1
fi

api="https://api.github.com/repos/$repo/labels"

labels=(
  "epic:e1|006b75|Epic E1 from docs/05-mvp-plan.md"
  "epic:e2|006b75|Epic E2 from docs/05-mvp-plan.md"
  "epic:e3|006b75|Epic E3 from docs/05-mvp-plan.md"
  "epic:e4|006b75|Epic E4 from docs/05-mvp-plan.md"
  "epic:e5|006b75|Epic E5 from docs/05-mvp-plan.md"
  "epic:e6|006b75|Epic E6 from docs/05-mvp-plan.md"
  "epic:e7|006b75|Epic E7 from docs/05-mvp-plan.md"
  "epic:e8|006b75|Epic E8 from docs/05-mvp-plan.md"
  "epic:e9|006b75|Epic E9 from docs/05-mvp-plan.md"
  "epic:e10|006b75|Epic E10 from docs/05-mvp-plan.md"
  "story|1d76db|A backlog item with a Definition of Ready"
  "ready|0e8a16|DoR met; an agent may pick it up"
  "blocked|b60205|Waiting on a decision or a dependency"
  "agent:go|1d76db|API, worker or sandbox work"
  "agent:web|5319e7|Web/UI work"
  "p0|b60205|Drop everything"
  "p1|d93f0b|Next"
  "p2|fbca04|Later"
  "spike|c2e0c6|Time-boxed investigation; needs an ADR before main"
  "triage|d4c5f9|Needs a decision or a DoR check"
)

for entry in "${labels[@]}"; do
  IFS='|' read -r name color description <<<"$entry"
  if [ "$dry_run" -eq 1 ]; then
    printf '%-10s #%s  %s\n' "$name" "$color" "$description"
    continue
  fi
  payload=$(printf '{"name":"%s","color":"%s","description":"%s"}' "$name" "$color" "$description")
  code=$(curl -sS -o /tmp/aip-label.json -w '%{http_code}' -X POST "$api" \
    -H "Authorization: Bearer $token" -H 'Accept: application/vnd.github+json' \
    -H 'Content-Type: application/json' -d "$payload")
  if [ "$code" = "201" ]; then
    echo "created  $name"
    continue
  fi
  if [ "$code" = "422" ]; then
    code=$(curl -sS -o /tmp/aip-label.json -w '%{http_code}' -X PATCH "$api/$name" \
      -H "Authorization: Bearer $token" -H 'Accept: application/vnd.github+json' \
      -H 'Content-Type: application/json' -d "$payload")
    if [ "$code" = "200" ]; then
      echo "updated  $name"
      continue
    fi
  fi
  echo "failed   $name: HTTP $code" >&2
  cat /tmp/aip-label.json >&2 || true
  exit 1
done
