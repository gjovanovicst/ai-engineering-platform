#!/usr/bin/env bash
# Pre-publication audit. This checks the *working tree/index*, not git history;
# run gitleaks/trufflehog over history separately before publishing a repo.
set -euo pipefail
cd "$(dirname "$0")/.."
bad=0
warn=0

fail() { echo "FAIL: $*" >&2; bad=1; }
warning() { echo "WARN: $*" >&2; warn=1; }

for required in LICENSE NOTICE SECURITY.md CONTRIBUTING.md CODE_OF_CONDUCT.md GOVERNANCE.md; do
  [ -f "$required" ] || fail "missing $required"
done

forbidden=$(git ls-files | grep -E '(^|/)\.env$|(^|/)jwt\.pem$|(^|/)backups/|\.zip$|\.pem$|(^|/)kubeconfig$' || true)
if [ -n "$forbidden" ]; then
  fail "tracked files that must not be public:"
  echo "$forbidden" | sed 's/^/  /' >&2
fi

patterns=(
  'BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY'
  'AKIA[0-9A-Z]{16}'
  'gh[pousr]_[A-Za-z0-9]{20,}'
  'glpat-[A-Za-z0-9_-]{20,}'
  'sk-[A-Za-z0-9]{24,}'
  'xox[baprs]-[A-Za-z0-9-]{10,}'
)
for pattern in "${patterns[@]}"; do
  # Test fixtures use obvious fake keys (AKIAABC...); real secrets are the
  # target here, so test files are excluded from this heuristic.
  hits=$(git grep -nI -E "$pattern" -- . ':!docs/demo-access.md' ':!*.example.yaml' ':!*_test.go' 2>/dev/null || true)
  if [ -n "$hits" ]; then
    fail "possible secret matching /$pattern/:"
    echo "$hits" | sed 's/^/  /' >&2
  fi
done

internal=$(git grep -nI -E '172\.19\.|192\.168\.|10\.[0-9]+\.[0-9]+\.' -- docs deploy 2>/dev/null || true)
if [ -n "$internal" ]; then
  warning "internal topology references found; review before publishing docs:"
  echo "$internal" | sed 's/^/  /' >&2
fi

large=$(git ls-files -z | while IFS= read -r -d '' f; do
  [ -f "$f" ] || continue
  size=$(wc -c < "$f" 2>/dev/null || echo 0)
  [ "$size" -gt 2097152 ] && printf '%s (%s bytes)\n' "$f" "$size"
done || true)
if [ -n "$large" ]; then
  warning "tracked files over 2 MiB:"
  echo "$large" | sed 's/^/  /' >&2
fi

if [ "$bad" -ne 0 ]; then
  echo "oss-audit: blocked ($bad)" >&2
  exit 1
fi
if [ "$warn" -ne 0 ]; then
  echo "oss-audit: warnings only; review before publishing" >&2
  exit 0
fi
echo "oss-audit: ok"
