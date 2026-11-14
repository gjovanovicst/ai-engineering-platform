#!/usr/bin/env bash
# Run govulncheck, allowing only the documented, unfixed crewjam/saml advisory.
# Any new vulnerability makes this fail.
set -euo pipefail
cd "$(dirname "$0")/../apps/api"
GOVULNCHECK_BIN=${GOVULNCHECK:-$(command -v govulncheck || true)}
if [ -z "$GOVULNCHECK_BIN" ]; then
  GOVULNCHECK_BIN="$(go env GOPATH)/bin/govulncheck"
fi
if [ ! -x "$GOVULNCHECK_BIN" ]; then
  echo "govulncheck is not installed" >&2
  exit 2
fi
set +e
out=$("$GOVULNCHECK_BIN" ./... 2>&1)
code=$?
set -e
if [ "$code" -eq 0 ]; then
  printf '%s\n' "$out"
  exit 0
fi
ids=$(printf '%s\n' "$out" | grep -oE 'GO-[0-9]{4}-[0-9]+' | sort -u || true)
if [ "$ids" = "GO-2026-6909" ]; then
  printf '%s\n' "$out"
  echo "govulncheck: accepted GO-2026-6909 (crewjam/saml; no upstream fix; see SECURITY.md)" >&2
  exit 0
fi
printf '%s\n' "$out"
exit 1
