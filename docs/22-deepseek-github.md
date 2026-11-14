# 22 · Real use: DeepSeek + GitHub

From zero to a merged pull request written by the agent, using **DeepSeek** as
the model (OpenAI-compatible API) and a real **GitHub** repository.

DeepSeek exposes `POST https://api.deepseek.com/chat/completions` with function
calling, which is exactly what the tool loop uses. The current model is
**`deepseek-flash`** (DeepSeek-V4.1-Flash); tool calls are supported. The legacy
names `deepseek-v4-flash` / `deepseek-v4-flash-vision-exp` are redirected to it.
`deepseek-chat` is no longer in the pricing table, so do not rely on it.

## 0. Prerequisites

| Item | Where |
|---|---|
| DeepSeek API key | <https://platform.deepseek.com> → API keys (top up a few dollars) |
| GitHub PAT | fine-grained token: **Contents r/w**, **Pull requests r/w**, **Metadata r** on one test repo |
| Test repo | e.g. `your-name/aip-e2e`, branch `main`, at least one commit |
| Tunnel for webhooks (optional) | `cloudflared tunnel --url http://localhost:18080` |

Never test against a production repository the first time.

## 1. Environment

```bash
export AIP_SECRETS_KEY=$(openssl rand -hex 32)   # encrypts provider keys at rest
export AIP_GITHUB_TOKEN=github_pat_xxx
export AIP_GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 16)
export AIP_GITHUB_CLONE_BASE=https://github.com
export AIP_GITHUB_API_URL=https://api.github.com

export AIP_AGENT_BACKEND=tools                   # real tool loop, not the echo stub
export AIP_LLM_MAX_ATTEMPTS=3

# deepseek-flash, PEAK rates, USD per 1K tokens (see the conversion below).
# Off-peak is half: input 0.00015, output 0.0006.
export AIP_PRICE_INPUT_PER_1K=0.0003
export AIP_PRICE_OUTPUT_PER_1K=0.0012
```

These prices are only the fallback for budgeting/costing; the authoritative
per-model price can be set in **Settings → Providers → Model prices**.

## 2. Start the stack

```bash
docker compose up -d postgres mailhog

cd apps/api
CGO_ENABLED=0 go build -o /tmp/aipd ./cmd/aipd
CGO_ENABLED=0 go build -o /tmp/aip-worker ./cmd/aip-worker
CGO_ENABLED=0 go build -o /tmp/aipctl ./cmd/aipctl
cd ../..

docker run --rm --network aip_default -v /tmp/aipctl:/aipctl:ro \
  -e AIP_DB_MIGRATOR_DSN="postgres://aip_migrator:migrator@postgres:5432/aip?sslmode=disable" \
  alpine:3.20 /aipctl migrate up

./scripts/seed-demo.sh
```

Run API and worker with the **same** GitHub/provider variables:

```bash
docker rm -f aip-demo-api aip-demo-worker 2>/dev/null || true

docker run -d --name aip-demo-api --network aip_default -p 18080:8080 \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_JWT_KEY=/jwt.pem -e AIP_WEB_URL=http://localhost:3000 \
  -e AIP_SMTP_ADDR=mailhog:1025 \
  -e AIP_SECRETS_KEY -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET \
  -e AIP_GITHUB_CLONE_BASE -e AIP_GITHUB_API_URL \
  -e AIP_PRICE_INPUT_PER_1K -e AIP_PRICE_OUTPUT_PER_1K \
  -v "$PWD/jwt.pem:/jwt.pem:ro" -v /tmp/aipd:/aipd:ro docker:27-cli /aipd

docker run -d --name aip-demo-worker --network aip_default \
  -e AIP_DB_DSN="postgres://aip_app:app@postgres:5432/aip?sslmode=disable" \
  -e AIP_SECRETS_KEY -e AIP_GITHUB_TOKEN -e AIP_GITHUB_WEBHOOK_SECRET \
  -e AIP_GITHUB_CLONE_BASE -e AIP_GITHUB_API_URL \
  -e AIP_AGENT_BACKEND -e AIP_LLM_MAX_ATTEMPTS \
  -e AIP_PRICE_INPUT_PER_1K -e AIP_PRICE_OUTPUT_PER_1K \
  -v /tmp/aip-worker:/aip-worker:ro docker:27-cli /aip-worker

./scripts/dev-web.sh --clean
```

Web: <http://localhost:3000> — `demo@acme.io` / `correct horse battery`.

## 3. Add DeepSeek as the provider

**Settings → Providers**:

| Field | Value |
|---|---|
| Name | `deepseek` |
| Base URL | `https://api.deepseek.com` |
| Model | `deepseek-flash` |
| API key | `sk-…` from DeepSeek |

The key is encrypted with `AIP_SECRETS_KEY` and never returned (UI shows
`key …abcd`). Then in **Model prices** add:

| Provider | Model | Input / 1K | Output / 1K |
|---|---|---|---|
| `openai-compatible` | `deepseek-flash` | `0.0003` | `0.0012` |

The provider name in the usage ledger is always `openai-compatible` (the protocol,
not the vendor), so the price row must use that value.

DeepSeek publishes prices **per 1M tokens**; the platform uses **per 1K**, so
divide by 1000:

| deepseek-flash | Per 1M (off-peak / peak) | Per 1K (off-peak / peak) |
|---|---|---|
| input, cache miss | 0.15 / 0.30 | 0.00015 / 0.0003 |
| input, cache hit | 0.003 / 0.006 | 0.000003 / 0.000006 |
| output | 0.60 / 1.20 | 0.0006 / 0.0012 |

Use the peak rates for budgets so a run cannot overspend; set the exact
cache-hit price only if you want precise cost reporting.

## 4. Connect the GitHub repository

**Projects → Novasoft CRM → Repositories**:

- provider `github`
- name `your-name/aip-e2e`
- default branch `main`
- webhook secret: the value of `AIP_GITHUB_WEBHOOK_SECRET`

The repo id for the webhook URL:

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:18080/api/v1/orgs/$ORG/projects/$PROJECT/repos"
```

GitHub → repo → Settings → Webhooks → Add webhook:

| Field | Value |
|---|---|
| Payload URL | `https://<tunnel-host>/webhooks/github/<repo-id>` |
| Content type | `application/json` |
| Secret | `AIP_GITHUB_WEBHOOK_SECRET` |
| Events | Pull requests |

## 5. The run

1. Create a task with **Given/When/Then** (required to reach `ready`).
2. Drag it to **Ready** on the board.
3. **Run agent** → planning run. With `AIP_AGENT_BACKEND=tools`, DeepSeek gets the
   tools `read_file`, `list_files`, `grep`, `write_file`, `run_tests`.
4. Open the run → **Approve plan** → the development run starts automatically.
5. Watch the log. On success: `aip/<run>` branch, PR, review findings.
6. **Pull requests → Approve → Merge** (human gate). Task becomes **Done**.

Budget example ($2 cap, 30 min):

```bash
curl -X POST "$API/orgs/$ORG/projects/$PROJECT/tasks/$TASK/runs" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"agent_type":"planning","budget":{"cost_micros":2000000,"wall_clock_s":1800}}'
```

On overrun: `budget_exceeded`, partial usage recorded, nothing pushed.

## 6. What to check afterwards

```bash
git clone https://github.com/your-name/aip-e2e && cd aip-e2e
git fetch origin && git log --oneline origin/aip/<run-id>
git diff main...origin/aip/<run-id>
```

In the app: Analytics → cost is `measured`; run details show token usage; audit
log records the approvals.

## 7. DeepSeek-specific troubleshooting

| Symptom | Cause / fix |
|---|---|
| `provider status 401` | key wrong or revoked; re-enter it (Rotate key) |
| `provider status 402` | DeepSeek balance empty; top up |
| model returns no tool calls | use `deepseek-flash` (tools are supported); `deepseek-v4-pro` also supports tools |
| `budget_exceeded` on the first step | limit smaller than one exchange; raise `cost_micros` |
| cost looks wrong | check the `openai-compatible` price row; DeepSeek lists prices per **1M** tokens, the platform uses per **1K** (divide by 1000) |
| no PR | GitHub token lacks `Contents`/`Pull requests`, or the clone failed |
| webhook ignored | secret mismatch or the repo id in the URL is wrong |

## 8. Limits of this build

- CI **check runs are not synced** yet; the webhook only handles the merge event.
- The agent's tool loop is bounded (12 steps) and non-interactive.
- No streaming to the UI for the tool loop (events are appended per step).
- A failed clone fails the run (`workspace_clone_failed`) by design.

Revoke the PAT and the DeepSeek key when you finish testing.
