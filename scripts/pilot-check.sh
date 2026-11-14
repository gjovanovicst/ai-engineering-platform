#!/bin/sh
# Repository-side pilot readiness. Environment-side items are in
# docs/17-pilot-checklist.md and cannot be checked from here.
set -u

FAILED=0
pass() { printf '  PASS  %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1"; FAILED=1; }
info() { printf '  INFO  %s\n' "$1"; }

printf '\n[pilot] repository checks\n'

if [ -z "$(git status --porcelain)" ]; then pass "git tree is clean"; else fail "git tree has uncommitted changes"; fi

if git ls-files --error-unmatch .env >/dev/null 2>&1; then fail ".env is committed"; else pass ".env is not committed"; fi
if git ls-files --error-unmatch jwt.pem >/dev/null 2>&1; then fail "jwt.pem is committed"; else pass "jwt.pem is not committed"; fi
if git ls-files --error-unmatch deploy/k8s/secret.yaml >/dev/null 2>&1; then fail "k8s secret.yaml is committed"; else pass "k8s secret.yaml is not committed"; fi

printf '\n[pilot] build and tests (make check)\n'
if make check >/tmp/aip-pilot-check.log 2>&1; then pass "make check"; else fail "make check (see /tmp/aip-pilot-check.log)"; fi

printf '\n[pilot] contract\n'
if git diff --quiet -- api/openapi.yaml apps/api/internal/httpapi/oapi; then pass "OpenAPI models in sync"; else fail "OpenAPI models are stale (run make generate)"; fi

printf '\n[pilot] migrations\n'
COUNT=$(ls apps/api/migrations/*.sql 2>/dev/null | wc -l | tr -d ' ')
info "$COUNT migration files; apply with aipctl migrate up and verify the version"
if grep -q "^-- +goose Down" apps/api/migrations/*.sql 2>/dev/null; then pass "migrations have a down path"; else fail "a migration has no down path"; fi

printf '\n[pilot] hygiene\n'
TODOS=$(grep -rn "TODO\|FIXME" apps/api/internal apps/web/src 2>/dev/null | wc -l | tr -d ' ')
if [ "$TODOS" = "0" ]; then pass "no TODO/FIXME"; else info "$TODOS TODO/FIXME markers (each needs an issue link)"; fi
if grep -rn "text-white\|#[0-9a-fA-F]\{6\}" apps/web/src/components 2>/dev/null | grep -v "from-brand\|from-ai" >/dev/null; then
  fail "possible raw colours in components"
else
  pass "no raw colours in components"
fi

printf '\n'
if [ "$FAILED" = "0" ]; then
  echo "[pilot] repository checks passed; continue with docs/17-pilot-checklist.md §2-§5"
else
  echo "[pilot] FAILED — fix the items above before a pilot"
  exit 1
fi
