# 21 · End-to-end test with a real GitHub repository

A complete manual run: from an empty database to a merged pull request opened by
the agent. Budget about 20 minutes; a real provider key costs a few cents.

## 0. What you need

- Docker and Go 1.25 (or the built binaries), Node 20 + pnpm
- A GitHub account and a **test repository** you own (never start with a
  production repo)
- A GitHub token: fine-grained PAT with **Contents: read/write**, **Pull
  requests: read/write**, **Metadata: read** on that repository
- An AI provider key (OpenAI-compatible endpoint)

## 1. GitHub repository

1. Create `your-name/aip-e2e` with a `main` branch and at least one commit.
2. Optional but recommended: protect `main` (require PR review, no direct push).
   The platform never pushes to the base branch, but protection proves the gate.

## 2. Start the stack with GitHub enabled

```bash
export AIP_GITHUB_TOKEN=ghp_xxx                  # the PAT above
export AIP_GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 16)
export AIP_GITHUB_CLONE_BASE=https://github.com
export AIP_AGENT_BACKEND=tools                   # real tool loop (needs a key)
export AIP_PRICE_INPUT_PER_1K=0.15
export AIP_PRICE_OUTPUT_PER_1K=0.60

docker compose up -d postgres mailhog
aipctl migrate up
./scripts/seed-demo.sh        # owner demo@acme.io + second admin
```

API and worker must see the same variables:

```bash
docker rm -f aip-demo-api aip-demo-worker 2>/dev/null || true
docker run -d --name aip-demo-api --network aip_default -p 18080:8080 \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_JWT_KEY=/jwt.pem -e AIP_WEB_URL=http://localhost:3000 \
  -e AIP_SMTP_ADDR=mailhog:1025 \
  -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET -e AIP_GITHUB_CLONE_BASE \
  -v "$PWD/jwt.pem:/jwt.pem:ro" -v /tmp/aipd:/aipd:ro docker:27-cli /aipd

docker run -d --name aip-demo-worker --network aip_default \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET -e AIP_GITHUB_CLONE_BASE \
  -e AIP_AGENT_BACKEND=tools \
  -e AIP_PRICE_INPUT_PER_1K -e AIP_PRICE_OUTPUT_PER_1K \
  -v /tmp/aip-worker:/aip-worker:ro docker:27-cli /aip-worker

./scripts/dev-web.sh --clean
```

Web: <http://localhost:3000> · login with the local demo account from `scripts/seed-demo.sh`.

## 3. Connect the repository

1. **Projects → Novasoft CRM → Repositories**
2. Provider `github`, name `your-name/aip-e2e`, default branch `main`
3. Webhook secret: the same value as `AIP_GITHUB_WEBHOOK_SECRET`
4. Connect. The panel lists the repo; note its **id**:

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:18080/api/v1/orgs/$ORG/projects/$PROJECT/repos"
```

## 4. GitHub webhook

Repository → Settings → Webhooks → Add webhook:

| Field | Value |
|---|---|
| Payload URL | `http://<public-host>:18080/webhooks/github/<repo-id>` |
| Content type | `application/json` |
| Secret | the value from step 3 |
| Events | Pull requests (add Check runs later) |

`localhost` is not reachable from GitHub; for a real test use a tunnel
(`cloudflared tunnel --url http://localhost:18080` or `ngrok http 18080`).

## 5. Add the provider key

**Settings → Providers**: name `openai`, base URL `https://api.openai.com/v1`,
model `gpt-4o-mini`, your key. The key is encrypted; the UI shows `key …abcd`.

## 6. Run the loop

1. **Projects → Novasoft CRM → New task**: title + Given/When/Then criteria
   (criteria are required to move a task to `ready`).
2. Drag the card to **Ready** (or use the task's transition).
3. Click **Run agent** → a **planning** run starts. Wait for the plan.
4. Open the run → **Approve plan**. The development run starts automatically.
5. Watch the log: the agent clones the repo, reads files, edits, runs tests.
6. If tests fail the run stops with `test_gate_failed`; nothing is pushed.
7. On success: branch `aip/<run-id>`, PR/MR opened, review findings attached.
8. Open **Pull requests** in the project → **Approve** → **Merge** (human gate).
9. The task becomes **Done**; Analytics shows the run and its cost.

## 7. Verify from GitHub

```bash
git clone https://github.com/your-name/aip-e2e && cd aip-e2e
git fetch origin && git log --oneline origin/aip/<run-id>   # base + agent commit
git diff main...origin/aip/<run-id>                          # the actual change
```

On GitHub: the PR body links the run; checks (if configured) are visible; the
merge commit is attributed to the approver.

## 8. Budget check

Start a run with a budget and a deliberately tiny limit — it must fail with
`budget_exceeded` and write partial usage:

```bash
curl -X POST "$API/orgs/$ORG/projects/$PROJECT/tasks/$TASK/runs" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"agent_type":"planning","budget":{"tokens_in":200,"cost_micros":1000}}'
```

## 9. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| Run fails `workspace_clone_failed` | wrong `AIP_GITHUB_TOKEN`, repo name, or branch; test `git clone` on the host |
| PR not created | token lacks `Contents`/`Pull requests`; check API logs |
| Merge refused | PR approval missing, or branch protection blocks the app's user |
| Webhook 401 | secret mismatch between repo and `AIP_GITHUB_WEBHOOK_SECRET` |
| Webhook 404 | `<repo-id>` is wrong or belongs to another org |
| `budget_exceeded` immediately | limit smaller than one model step (normal) |
| Nav/CSS look stale | `./scripts/dev-web.sh --clean` then hard refresh (`Ctrl+Shift+R`) |
| 429 on login | 5 attempts/minute; wait |

## 10. Cleanup

```bash
docker rm -f aip-demo-api aip-demo-worker
docker compose down            # keeps the database volume
# full reset: docker compose down -v
```

Revoke the PAT and the provider key when the test is over.
