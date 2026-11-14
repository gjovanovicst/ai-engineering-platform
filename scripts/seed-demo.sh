#!/usr/bin/env bash
# Creates (or reuses) the local demo organisation and prints its credentials.
# Idempotent: safe to run against a database that already has demo data.
set -euo pipefail

API="${AIP_API:-http://localhost:18080/api/v1}"
DEMO_EMAIL="${DEMO_EMAIL:-demo@acme.io}"
DEMO_PASSWORD="${DEMO_PASSWORD:-correct horse battery}"
ADMIN_EMAIL="${ADMIN_EMAIL:-second@acme.io}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-another strong pass}"
PROJECT_NAME="${PROJECT_NAME:-Novasoft CRM}"

json() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }

# post fails loudly: a silent 4xx here used to hide broken seeding.
post() {
  local url="$1" data="$2" response code body
  local auth=()
  if [ -n "${TOKEN:-}" ]; then auth=(-H "Authorization: Bearer $TOKEN"); fi
  response=$(curl -s -w '\n%{http_code}' -X POST "$url" \
    -H 'Content-Type: application/json' "${auth[@]}" --data "$data")
  code=${response##*$'\n'}
  body=${response%$'\n'*}
  if [ "$code" -ge 300 ]; then
    echo "POST $url -> $code: $body" >&2
    return 1
  fi
  printf '%s' "$body"
}

json() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }


status=$(curl -s -o /tmp/seed-body.json -w '%{http_code}' -X POST "$API/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$DEMO_EMAIL\",\"name\":\"Demo Owner\",\"password\":\"$DEMO_PASSWORD\",\"org_name\":\"Acme\"}")
if [ "$status" != "201" ]; then
  echo "demo user exists (register returned $status), logging in"
fi

TOKEN=$(post "$API/auth/login" "{\"email\":\"$DEMO_EMAIL\",\"password\":\"$DEMO_PASSWORD\"}" | json "d['access_token']")
ORG=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/auth/me" | json "d['org_id']")

PROJECTS=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/orgs/$ORG/projects")
PROJECT_ID=$(echo "$PROJECTS" | json "d['data'][0]['id'] if d['data'] else ''")
if [ -z "$PROJECT_ID" ]; then
  PROJECT_ID=$(post "$API/orgs/$ORG/projects" "{\"name\":\"$PROJECT_NAME\",\"description\":\"Demo CRM used to show the ASDLC loop\"}" | json "d['id']")
  echo "created project $PROJECT_NAME"
fi

TASKS_JSON=$(curl -s -H "Authorization: Bearer $TOKEN" "$API/orgs/$ORG/projects/$PROJECT_ID/tasks")
ensure_task() {
  local title="$1" body="$2"
  exists=$(echo "$TASKS_JSON" | TITLE="$title" python3 -c 'import os,sys,json;print(any(t["title"]==os.environ["TITLE"] for t in json.load(sys.stdin)["data"]))')
  if [ "$exists" != "True" ]; then
    post "$API/orgs/$ORG/projects/$PROJECT_ID/tasks" "$body" >/dev/null
    echo "created task: $title"
  fi
}
ensure_task "Add phone field to the customer form" '{"title":"Add phone field to the customer form","description":"Customers ask for a phone number on the create/edit form. Add it with validation and a test.","priority":"high","tags":["demo","frontend"]}'
ensure_task "Expose customer search API" '{"title":"Expose customer search API","description":"Add GET /customers?q= returning matches by name and email, with pagination.","priority":"high","tags":["demo","api"]}'
ensure_task "Document the deployment runbook" '{"title":"Document the deployment runbook","description":"Write the runbook for staging and prod deploys, including the four-eyes step.","priority":"med","tags":["demo","docs"]}'

for env in dev prod; do
  curl -s -o /dev/null -X POST "$API/orgs/$ORG/projects/$PROJECT_ID/environments" \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d "{\"name\":\"$env\",\"url\":\"https://$env.demo.example.com\"}" || true
done

admin_login=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/auth/login" \
  -H 'Content-Type: application/json' -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")
if [ "$admin_login" = "200" ]; then
  echo "second admin already exists"
else
  INVITE=$(post "$API/orgs/$ORG/invites" "{\"email\":\"$ADMIN_EMAIL\",\"role\":\"admin\"}")
  INVITE_TOKEN=$(echo "$INVITE" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('token',''))" 2>/dev/null || true)
  if [ -n "$INVITE_TOKEN" ]; then
    accept=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/invites/accept" \
      -H 'Content-Type: application/json' \
      -d "{\"token\":\"$INVITE_TOKEN\",\"name\":\"Second Admin\",\"password\":\"$ADMIN_PASSWORD\"}")
    echo "second admin accept -> $accept"
  else
    echo "could not create the second admin invite: $INVITE" >&2
  fi
fi

for cred in "$DEMO_EMAIL:$DEMO_PASSWORD" "$ADMIN_EMAIL:$ADMIN_PASSWORD"; do
  email=${cred%%:*}; password=${cred#*:}
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/auth/login" \
    -H 'Content-Type: application/json' -d "{\"email\":\"$email\",\"password\":\"$password\"}")
  echo "login $email -> $code"
done

cat <<SUMMARY

Demo access
  web      http://localhost:3000
  api      $API
  org      $ORG
  project  $PROJECT_ID ($PROJECT_NAME)

  owner        $DEMO_EMAIL / $DEMO_PASSWORD
  second admin $ADMIN_EMAIL / $ADMIN_PASSWORD

These are local demo credentials only. Never reuse them outside the demo.
SUMMARY
