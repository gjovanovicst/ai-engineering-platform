# 02 · Architecture (Go)

## 1. Requirements that shape the architecture

| Requirement | Consequence |
|---|---|
| A run lasts minutes to hours, must survive a restart | durable state machine in Postgres + River job queue, checkpoint per phase |
| Live log/diff in the UI | SSE with replay from `run_events` (seq per run) |
| Multi-tenant | `org_id` on every aggregate + RLS as a second line of defense |
| The agent must not touch production | isolated sandbox per run, no docker socket, egress allowlist |
| Reproducible audit | append-only `run_events` + `audit_log`, hash of artifacts |
| Cost must be controlled | budget on the run, `usage_ledger`, cancel before overage |
| Backend in Go | everything server-side is Go; no Python/Node services in the runtime path |

**Non-goals:** horizontal scalability beyond 100 concurrent runs in the MVP;
multi-region; event sourcing of the whole domain (only runs are event-sourced).

## 2. Context

```
┌──────────┐   HTTPS/SSE   ┌──────────────┐
│  web     │ ────────────▶ │  aipd (API)  │
│ Next.js  │ ◀──────────── │  Go, chi     │
└──────────┘               └──────┬───────┘
                                  │ enqueue (River)
                           ┌──────▼────────┐        ┌─────────────┐
                           │  PostgreSQL   │◀──────▶│  aip-worker │
                           │  (state+queue)│        │  Go runtime │
                           └──────┬────────┘        └──┬───────┬──┘
                                  │                    │       │
                           ┌──────▼──────┐      ┌──────▼──┐ ┌──▼─────────┐
                           │   Redis     │      │ Sandbox │ │ LLM APIs   │
                           │ SSE fanout  │      │ Docker/ │ │ OpenAI-    │
                           └─────────────┘      │ K8s job │ │ compat,    │
                                                └────┬────┘ │ Anthropic  │
                                                     │      └────────────┘
                                                ┌────▼────┐
                                                │ S3/MinIO│  (diff, log, plan)
                                                └─────────┘
```

`aipd` never executes an agent. `aip-worker` never serves HTTP to the user
(except health/metrics). This separation is deliberate: the API scales with the number of users,
the worker with the number of runs.

## 3. Layers and the dependency rule

```
cmd/            → internal/httpapi, internal/app, internal/adapters
internal/httpapi→ internal/app, internal/domain
internal/app    → internal/domain, port interfaces (defined in app/ports.go)
internal/adapters→ internal/app (implements the ports), internal/domain
internal/domain → nothing (stdlib only)
internal/agent  → internal/domain, ports (LLM, sandbox, git, artifacts)
```

Rule: **the arrows point toward the domain**. `domain` does not import `pgx`, `chi`, `aws-sdk`.
Ports are interfaces in `internal/app/ports.go` (the consumer side), not in the adapters.

```go
// internal/app/ports.go
type TxManager interface {
    InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type RunRepo interface {
    Create(ctx context.Context, r *domain.Run) error
    GetForUpdate(ctx context.Context, id domain.RunID) (*domain.Run, error)
    Transition(ctx context.Context, id domain.RunID, to domain.RunStatus, ev domain.Event) error
}

type Provider interface {
    Name() string
    Stream(ctx context.Context, req CompletionRequest) (Stream, error)
    Models(ctx context.Context) ([]ModelInfo, error)
}

type Sandbox interface {
    Create(ctx context.Context, spec SandboxSpec) (SandboxRef, error)
    Exec(ctx context.Context, ref SandboxRef, cmd ExecRequest) (ExecResult, error)
    Upload(ctx context.Context, ref SandboxRef, path string, r io.Reader) error
    Destroy(ctx context.Context, ref SandboxRef) error
}

type ArtifactStore interface {
    Put(ctx context.Context, key string, r io.Reader, meta ArtifactMeta) (ArtifactRef, error)
    Get(ctx context.Context, ref ArtifactRef) (io.ReadCloser, error)
}
```

## 4. Package structure

```
cmd/
  aipd/main.go              # HTTP, graceful shutdown, OTel
  aip-worker/main.go        # River client, worker pool, signals
  aipctl/main.go            # migrate, seed, replay, rotate-key

internal/
  config/                   # env → Config, validation at startup (fail fast)
  domain/
    org.go user.go project.go task.go run.go phase.go approval.go
    blueprint.go rule.go repo.go env.go usage.go errors.go
  app/
    ports.go                # interfaces to the outside
    auth/                   # login, refresh, OIDC, sessions
    orgs/ projects/ tasks/  # use cases + validation + authorization
    runs/                   # StartRun, Approve, Cancel, GetStream
    approvals/ rules/ blueprints/ repos/ envs/ analytics/
  agent/
    runtime.go              # loop: phase → tool → checkpoint
    phases/                 # context, plan, implement, test, review, diff, pr
    tools/                  # fs, shell, git, test, http_fetch, search
    prompt/                 # templates (embed) + render + hash
    budget.go               # token/cost/time guard
  adapters/
    postgres/               # pgx, sqlc-generated, repo implementations
    llm/{openai,anthropic}/ # streaming adapters
    sandbox/{docker,k8s}/
    gitx/                   # go-git + CLI in the sandbox
    github/                 # App auth, PR, checks, webhooks
    s3/ redis/ oidc/
  httpapi/
    router.go middleware.go handlers/ dto/ oapi/ sse.go
  platform/
    log/ config/ clock/ idgen/ httpsse/ retry/ validate/
migrations/                 # goose, embedded into the binary

# repository root:
# api/openapi.yaml          # contract; embedded and served at /openapi.yaml
```

## 5. Request flow: "start an agent on a task"

1. `POST /api/v1/orgs/{org}/tasks/{task}/runs` (Idempotency-Key required).
2. The handler validates the body, `app/runs.StartRun` in a single transaction:
   - tenancy + RBAC check + task.status ∈ {`ready`},
   - `SELECT ... FOR UPDATE` on the task (prevents two active runs),
   - creation of `runs` (status `queued`, `budget`, `model`, `agent_type`, `rules_snapshot`),
   - `INSERT` of the initial `run_events(run.started)` with `seq=1`,
   - enqueue River job `agent_run.execute` with `run_id` (unique per run).
3. The API returns `202 Accepted` + `Location: /runs/{id}`.
4. The worker picks up the job, transitions `queued → running`, and executes the phases.
5. Every event: `INSERT run_events` + `PUBLISH redis channel run:{id}`.
6. The UI listens on `GET /runs/{id}/events` (SSE); reconnect with `Last-Event-ID: {seq}`.

## 6. Agent runtime

> Detailed specification (tool schemas, prompt layers, context assembly, sandbox image):
> [11-agent-runtime.md](11-agent-runtime.md). This section describes where the code lives, not the contract.


### 6.1 Loop

```go
func (r *Runtime) Execute(ctx context.Context, run *domain.Run) error {
    for {
        select {
        case <-ctx.Done():      // cancel or shutdown
            return r.checkpoint(ctx, run, domain.RunCanceled)
        default:
        }

        ph := r.nextPhase(run)
        if ph == nil { return r.complete(ctx, run) }

        if err := r.budget.Check(run); err != nil {
            return r.fail(ctx, run, domain.ErrBudgetExceeded, err)
        }
        if err := r.enterPhase(ctx, run, ph); err != nil { return err }

        out, err := r.runPhase(ctx, run, ph)   // LLM + tool calls
        if err != nil {
            return r.handlePhaseError(ctx, run, ph, err)
        }
        if err := r.exitPhase(ctx, run, ph, out); err != nil { return err }

        if ph.NeedsApproval() {
            if err := r.awaitApproval(ctx, run, ph, out); err != nil { return err }
        }
    }
}
```

### 6.2 Checkpoint

A checkpoint is **one** `UPDATE runs SET phase=$, state=$` + `INSERT run_phases` in the same
transaction as `run_events`. If the worker crashes after `exitPhase`, the restart sees `phase`
and continues from the next one; if it crashes inside a phase, the phase is repeated (which is
why tool calls are idempotent per `(run_id, tool_call_seq)`).

### 6.3 Budget

```go
type Budget struct {
    MaxTokensIn, MaxTokensOut int64
    MaxCostMicros             int64
    MaxWallClock              time.Duration
}
```

Check before **every** LLM call (prompt estimate) and after it (actual usage).
`usage_ledger` is written in the same transaction as the provider response — there is no "lost"
cost and no double charge on retry (key: `run_id + attempt`).

### 6.4 Tool allowlist

Per agent type, from the blueprint:

```yaml
tools:
  planning:    [fs.list, fs.read, search.code, git.status, plan.submit]
  development: [fs.list, fs.read, fs.write, search.code, git.status, git.diff,
                git.commit, shell.exec, test.run]
  testing:     [fs.read, fs.write(test/**), test.run, shell.exec(go test|pnpm test)]
  review:      [fs.read, git.diff, search.code]
```

`shell.exec` is not an "arbitrary command": prefix allowlist + ban on `;`, `|`, `>`
outside explicitly allowed patterns. A rejected call is `403 tool_not_permitted` in
`run_events` — and that is an audit fact, not an error to ignore.

### 6.5 Concurrency and fair scheduling

- Worker pool: `AIP_WORKER_CONCURRENCY` (default 8) River jobs.
- Per-org limit of active runs (`orgs.max_concurrent_runs`, default 4) — enforced
  in `StartRun` (fast fail `429 concurrency_limit`) and in the worker (re-enqueue with delay).
- One run = one goroutine + one sandbox. No shared state between runs.

## 7. LLM provider abstraction

```go
type CompletionRequest struct {
    Model       string
    System      string
    Messages    []Message
    Tools       []ToolSpec
    MaxTokens   int
    Temperature float64
    Stream      bool
}

type Stream interface {
    Recv() (Chunk, error)   // io.EOF at the end
    Close() error
}
```

Adapters: `openai` (also works for any OpenAI-compatible endpoint via `base_url`),
`anthropic`. Registered by `provider_id` from the `ai_providers` table. Model metadata
(context window, price per 1M tokens) is in the `model_prices` table with `valid_from` —
the price is **frozen** on the run at start time, so the bill does not "jump" retroactively.

Retry policy: 429/500/502/503/504 → backoff 1s, 2s, 4s, 8s, 16s (jitter ±20%), max 5.
Timeout per call: 120s (stream), 600s (entire run step).

## 8. Sandbox

```go
type SandboxSpec struct {
    Image      string            // aip/sandbox:go1.25-node22
    Workdir    string            // /workspace
    RepoURL    string
    Ref        string
    Env        map[string]string // no secrets; secrets go through the egress proxy
    Limits     Limits            // CPU 2, Mem 4Gi, Disk 10Gi, PIDs 512
    Timeout    time.Duration     // 30m
    EgressAllow []string         // proxy.golang.org, registry.npmjs.org, api.github.com
}
```

Implementations:
- `docker` — dev/self-host: `--network=aip-egress`, read-only rootfs + tmpfs `/tmp`,
  non-root uid 10001, `--cap-drop=ALL`, `--security-opt=no-new-privileges`.
- `k8s` — production: Job with `activeDeadlineSeconds`, `automountServiceAccountToken:false`,
  NetworkPolicy, ephemeral PVC, `restartPolicy: Never`.

The sandbox is **always** destroyed at the end of the run (`defer`), and that is idempotent. Artifacts
are collected before destruction (diff, log, test report) and go to S3.

## 9. Git (GitHub and GitLab)

- Clone in the sandbox via the `git` CLI (supports shallow + sparse for large repos).
- Commit: one commit per run, message `aip: {task.ref} {task.title}` + `Run-Id` trailer.
- Push: branch `aip/{run_id_short}`.
- PR: through the `git.Provider` port (ADR-0007); GitHub App installation token and GitLab project token are adapter details; body = plan + test report + run link.
- Checks: `POST /repos/{owner}/{repo}/check-runs` with `external_id = run_id`.
- Webhook (`POST /webhooks/github`): `pull_request.closed` → `task.done`; `check_run` →
  `test_results`; verification of `X-Hub-Signature-256` (HMAC, constant-time compare).

## 10. Auth, RBAC, tenancy

- Session: access JWT (15 min, EdDSA) + refresh token (30 days, rotating, hash in
  `sessions`). Refresh reuse detects theft and kills all of the user's sessions.
- OIDC (GitHub, Google) + email/password (`argon2id`, params in `internal/app/auth`).
- Role: `owner` > `admin` > `member` > `viewer`. Checked in **one** middleware
  (`requireRole`) + in the use case for sensitive actions (defense in depth).
- Tenancy: every query carries `org_id`; Postgres RLS policies on all tenant tables
  (`SET LOCAL app.org_id` per transaction). An adapter that forgets `org_id` fails on RLS.
- API keys (`aip_...`): hash + scopes, for CI integrations; they are never logged.

## 11. Real-time (SSE)

```
GET /api/v1/orgs/{org}/runs/{id}/events
Accept: text/event-stream
Last-Event-ID: 42
```

- The API reads `run_events WHERE run_id=$1 AND seq > $2 ORDER BY seq` (backfill), then
  subscribes to the Redis channel `run:{id}`.
- `id:` = `seq`, `event:` = event type, `data:` = JSON (see [api/events.md](../api/events.md)).
- Heartbeat `: ping` every 15s; cancel after 30s without a reader; max 1h per connection.
- If Redis is unavailable — degrade to polling (the client has a fallback), the API does not fail.

## 12. Errors and validation

- The domain returns typed errors (`domain.ErrNotFound`, `ErrConflict`, `ErrInvalidTransition`).
- HTTP mapping in one place (`httpapi/errors.go`) → RFC 9457 `application/problem+json`:
  `{type, title, status, detail, instance, code, fields?}`.
- Input validation: `oapi-codegen` generates the types; manual validation only for business
  rules. No handler does `json.Unmarshal` by hand.

## 13. Observability

- OTel trace per request and per run; span hierarchy: `run` → `phase` → `llm.call` /
  `tool.exec`, with attributes `run.id`, `org.id`, `phase`, `model`, `tokens`.
- Prometheus: `aip_runs_active`, `aip_run_duration_seconds`, `aip_llm_tokens_total`,
  `aip_llm_cost_micros_total`, `aip_sandbox_start_seconds`, `aip_queue_depth`.
- `slog` JSON with `request_id`, `org_id`, `user_id`, `run_id`; **never** prompt content
  at INFO level (only at DEBUG, and without secrets).
- A log line is a structure, not a string: `slog.String("event","run.phase.failed")`.

## 14. Configuration and migrations

- `internal/config`: everything from env with the `AIP_` prefix, validation at startup, `String()` with
  redaction of secrets. No config file read at runtime (12-factor).
- Migrations: `goose` + `embed.FS`, run by `aipctl migrate` or `aipd --migrate` in CI/CD.
  Migrations are **forward-only**; rollback is a new migration.
- Feature flags: table `flags` (org-level override), read through `app/flags` with a 30s cache.

## 15. Performance (MVP goals)

| Operation | p95 goal | Note |
|---|---|---|
| `GET /tasks` (50 items) | < 80 ms | index `(project_id, status, updated_at)` |
| `POST /runs` | < 150 ms | 1 transaction, no LLM call |
| SSE first event | < 300 ms | backfill + subscribe |
| Sandbox start | < 4 s | warm image, pre-pull at worker start |
| Dashboard aggregates | < 250 ms | materialized view `mv_org_daily` (refresh 5 min) |

## 16. Deployment

- Multi-stage Dockerfile: `golang:1.25-alpine` build → `gcr.io/distroless/static` runtime,
  `CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`.
- `docker compose`: `postgres:17`, `redis:7`, `minio`, `aipd`, `aip-worker`, `web`.
- K8s: 2 deployments (`aipd` HPA on CPU/RPS, `aip-worker` HPA on `aip_queue_depth`),
  `PodDisruptionBudget`, `NetworkPolicy`, secrets from SealedSecret/Vault.
- CI: `make check` → build → `openapi-diff` → image scan (trivy) → push.
- Graceful shutdown: `aipd` 20s drain; `aip-worker` stops taking jobs, waits for
  in-flight ones for up to 15 min, then checkpoint + re-enqueue.

## 17. Decisions (ADR index)

[ADR-0001 Go](adr/README.md#adr-0001--backend-in-go) ·
[ADR-0002 Postgres+River](adr/README.md#adr-0002--postgres-as-a-queue-river) ·
[ADR-0003 SSE](adr/README.md#adr-0003--sse-instead-of-websocket) ·
[ADR-0004 Sandbox](adr/README.md#adr-0004--sandbox-per-run) ·
[ADR-0005 oapi-codegen](adr/README.md#adr-0005--openapi-first--oapi-codegen) ·
[ADR-0006 RLS](adr/README.md#adr-0006--tenancy-through-org_id--rls)

## 18. Bootstrap and seed

The first user cannot be created through the API (there is no one to send an invite to). That is why there is
a CLI path, explicitly separated from demo data:

```bash
aipctl bootstrap --org "Acme" --slug acme --email goran@acme.io --name "Goran"
aipctl bootstrap --status                     # check without changes
```

- Idempotent: if the org or the user exists → `already` (exit 0), nothing is changed.
- Creates: `orgs` + `users` (`email_verified_at = now()`) + `memberships(owner)`.
- Generates a one-time password (32B base32), prints it **to stdout only**, and requires a change
  at first sign-in (`users.must_change_password = true`).
- Writes `audit_log(action='org.bootstrap')` with `actor_kind='system'`.
- Never runs in production without `--confirm` (env `AIP_ENV=prod`).

Demo data is separate: `aipctl seed --demo` ([07-testing.md §9](07-testing.md#9-test-data)).
Seed never modifies existing accounts and can be run more than once.
