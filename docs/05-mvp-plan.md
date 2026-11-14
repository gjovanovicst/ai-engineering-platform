# 05 · MVP plan

Story format: `E{epik}-S{n}` · title · acceptance criteria (AC) · dependencies.
A story is done when it passes [DoD](#definition-of-done) — not when "it works on my machine".

Estimate: `S` ≤ 0.5 agent-days, `M` ≤ 1 day, `L` > 1 day (must be split).
The order is mandatory: a milestone does not start before the previous one passes its exit criteria.

## Status (2026-10-22)

This is the implementation state, not the plan. A story is ✅ only when its AC and
DoD are met in the repository and `make check` (plus `make e2e` for flows) is green.

| Area | State | Evidence |
|---|---|---|
| M0 foundation: config, DB + RLS, migrations, domain, OpenAPI | ✅ | `make check`, migrations 0001–0015 |
| Auth: register, verify, login, refresh rotation, reset, sessions, rate limits | ✅ | `internal/app/auth`, handler tests |
| Invites (org-scoped) + email + UI | ✅ | `internal/app/invites`, Mailhog e2e |
| OIDC (GitHub, Google) | ✅ | `internal/app/oidc`, fake-provider tests; no live OAuth app tested |
| Projects, tasks (+ state machine, DoR), task drawer | ✅ | `internal/app/{projects,tasks}`, web pages |
| Runs: state machine, event log, SSE, cancel | ✅ | `internal/app/runs`, `internal/httpapi/runs.go` |
| River queue, worker, recovery, cooperative cancel | ✅ | `internal/platform/queue`, `cmd/aip-worker` |
| Agent backend + Docker sandbox | ✅ | `internal/agent`, Docker integration test |
| Sandbox image (Go/Node/git/ripgrep) | ✅ | `deploy/sandbox/Dockerfile`, `make sandbox-image` |
| Kubernetes Job-per-run sandbox driver | ✅ (fake-clientset tests; not run on a live cluster) | `sandbox/kubernetes.go`, `deploy/k8s` |
| Planning: plan artifact, `awaiting_approval`, decisions | ✅ | migration 0008, `runs.Decide` |
| Development: diff, test gate, review agent, findings | ✅ | migrations 0009/0010, `make e2e` |
| Git: GitHub + GitLab adapters, repo connect, webhook merge | ✅ | `internal/adapters/git`, migrations 0011/0012 |
| Branch push + PR/MR creation (host push) | ✅ | `git.HostPusher`, `git.Publisher`; real remote not used in tests |
| PR approval gate + auto-merge | ✅ | `runs.Decide` (pr), webhook gate, `MergePullRequest` |
| Blueprints: validation, draft/publish, immutability, instantiate | ✅ | migration 0013/0014, `/blueprints` UI |
| Analytics: lead time, approval time, rework, merged PRs | ✅ | `internal/app/analytics`, `/analytics` |
| Observability: OTLP spans + runbook | ✅ | `internal/platform/telemetry`, `docs/14-runbook.md` |
| Web: login/register, projects, tasks, runs, diff, approvals, repos, settings, blueprints, analytics | ✅ | `apps/web`, 11 routes |
| CI: unit + integration + OpenAPI check + e2e | ✅ | `.github/workflows/ci.yml`, `scripts/e2e.sh` |
| Blueprint wizard (5 steps: basics, stack, repository, testing, agents + review) | ✅ | `/blueprints` wizard |
| Tasks generated from a blueprint | ⬜ | schema v2 needed (ADR) |
| Blueprint version change requests | ⬜ | only version 1 exists; published specs are immutable |
| Search across tasks | ✅ | `GET /orgs/{org}/search`, `/search` page |
| Repositories page (org-wide) | ✅ | `GET /orgs/{org}/repos`, `/repositories` |
| Rules: versioned editor + prompt injection | ✅ | migration 0016, `internal/app/rules`, Settings → Rules, `rules.snapshot` event |
| Design system + components registry | ✅ | editable registries (migration 0021): tokens CRUD, component contracts CRUD, seed from baseline |
| Environments + deployments + prod four-eyes | ✅ | migration 0017, `internal/app/deployments`, project panel, approve endpoint |
| Providers settings (encrypted key, `has_key`) | ✅ | migration 0018, AES-256-GCM at rest, Settings → Providers |
| Provider-backed runs + usage ledger + cost | ✅ | `AIP_AGENT_BACKEND=provider`, migration 0019, Analytics `cost_usd` measured |
| Per-model price table | ✅ | migration 0020, `/orgs/{org}/model-prices`, Settings → Providers → Model prices |
| Streaming + retry/backoff (provider runs) | ✅ | SSE streaming, 429/5xx retries with jitter (`AIP_LLM_MAX_ATTEMPTS`) |
| Provider key rotation, enable/disable, delete | ✅ | rotate/patch/delete endpoints + Settings actions |
| Agents page (org-wide runs, status filter) | ✅ | `GET /orgs/{org}/runs`, `/agents` |
| Members + audit settings | ✅ | `GET /orgs/{org}/members`, `/orgs/{org}/audit`, Settings tabs |
| Threat model | ✅ | `docs/15-security.md` |
| Metric definitions (G10) | ✅ | `docs/16-metrics.md` |
| Alert rules + SLO definitions | ✅ | `deploy/observability/` (Prometheus rules, sql_exporter, Grafana) |
| Automated backups + restore drill | ✅ | `scripts/backup.sh`, `restore-check.sh`, K8s CronJob (drill executed against compose) |
| Pilot readiness checklist | ✅ | `docs/17-pilot-checklist.md`, `make pilot-check` |
| Load/soak test | ✅ | `make loadtest`, baseline in `docs/18-load-testing.md` |
| Accessibility audit | ✅ | `docs/19-a11y.md`; axe runs on built public pages in CI; focus trap in the drawer |
| Invite accept without an account | ✅ | `POST /invites/accept` creates the user + membership and returns a session |

## F0 · Preparation (before the first story)

**Exit:** tools work, the repo exists with branch protection, decisions from [13-setup.md](13-setup.md) confirmed.

| # | Task | Check |
|---|---|---|
| 1 | Go 1.25+, Node 22, pnpm 9, Docker, `golangci-lint`, `goose`, `sqlc`, `oapi-codegen`, `gh` | `make tools-check` (added in E1-S1) |
| 2 | GitHub repo + branch protection (`main`: PR + `make check`, no force-push) | `gh api repos/.../branches/main/protection` |
| 3 | GitHub App (M2) and OIDC app (M0) registered | `client_id` in `.env` |
| 4 | Provider keys (OpenAI-compatible + Anthropic) | `curl` to `/models` returns 200 |
| 5 | Mailhog locally; Postmark account for staging | test email arrives |
| 6 | Decisions: module path, domains, email sender | recorded in `13-setup.md` §1–§6 |

Duration: 0.5–1 day. Not counted in the 8-week slice.

## M0 · Walking skeleton (foundation)

**Exit:** `docker compose up` brings up API + worker + web; sign-in works; a project and a task
can be created; `/healthz` green; CI passes `make check`.

| ID | Story | AC | Dep |
|---|---|---|---|
| E1-S1 | Repo skeleton + Makefile + CI | `make build test lint` works locally and in CI; golangci-lint config; branch protection documented | — M |
| E1-S2 | Config + slog + OTel bootstrap | invalid env → the process exits with a clear message; `/metrics` exposes `aip_build_info` | S |
| E1-S3 | Postgres + goose migrations + RLS | `make migrate-up` idempotent; the `aip_app` role does not see another `org_id` (test) | M |
| E1-S4 | Domain: org/user/project/task | invariants in `domain` with unit tests; no I/O in the package | M |
| E1-S5 | Auth: email+password, JWT, refresh, CSRF | per [12-auth.md](12-auth.md): register/verify/login/reset/invite/sessions; refresh reuse kills sessions; `argon2id`; CSRF for cookie auth | L |
| E1-S6 | RBAC middleware | the table from [04-api.md](04-api.md#4-rbac) covered by a test per role | S |
| E1-S7 | CRUD org/project/task + OpenAPI handlers | `oapi-codegen` generates the types; `openapi-diff` in CI fails on a breaking change | M |
| E2-S1 | Web shell: AppShell + tokens + theme | tokens from [06-design-system.md](06-design-system.md#1-design-tokens); light/dark; no hardcoded colors (lint rule) | M |
| E2-S2 | Sign-in + project list + task list | loading/empty/error states; a11y focus ring | M |
| E2-S3 | Primitives: EmptyState, Alert, Modal, Toast, Skeleton | all states from [§5.16](06-design-system.md#516-empty-states-modals-toast); toast `role=status/alert`; focus trap in the modal | M |

## M1 · Agent loop (the heart of the product)

**Exit:** a planning run is started from the UI, the live log is visible, the plan is approved, the
development agent produces a diff in the sandbox, the tests run, the diff is visible. The run
survives a worker restart.

| ID | Story | AC | Dep |
|---|---|---|---|
| E3-S0 | Agent runtime spec → code (`internal/agent`) | tool registry with 12 schemas; prompt layers + hash; deterministic context assembly (same input → same manifest) | L |
| E3-S0a | `AgentBackend` adapter (Claude Code / Codex / OpenHands) | `Start/Stream/Cancel` interface; one adapter completes a whole task on the test repo; the sandbox stays ours; I/O is recorded for replay | M |
| E3-S1 | `runs` model + state machine + `run_events` | illegal transition → `409`; one active run per task (DB index test) | M |
| E3-S2 | River job `agent_run.execute` + retry | worker crash → the run resumes from the checkpoint (test with kill -9) | M |
| E3-S3 | Provider adapters (openai-compatible, anthropic) | streaming; 429/5xx retry with backoff; the price is frozen on the run | M |
| E3-S4 | Context assembly + `rules_snapshot` | the manifest contains files + rule versions + hash; deterministic for the same input | L |
| E3-S5 | Planning agent + plan artifact | the plan is markdown with steps/files/risks; `approval.type=plan` | M |
| E3-S6 | Approval flow (request/decide/expire) | approve unblocks `implement`; reject → run `failed` with a reason; 72h expiry | M |
| E3-S7 | Sandbox adapter (docker) | run without network except the allowlist; non-root; limits; `Destroy` idempotent | L |
| E3-S8 | Development agent + diff + commit | one commit per run; `Run-Id` trailer; diff in S3 with a hash | L |
| E3-S9 | Test phase + `test_results` | parses `go test -json` and `vitest --reporter=json`; G2 thresholds from the blueprint | M |
| E3-S10 | SSE endpoint + backfill | reconnect with `Last-Event-ID` does not duplicate events; heartbeat | M |
| E4-S1 | Dashboard (light) | KPI + approvals + active runs; every metric has a `source` | M |
| E4-S2 | Agent Run Details (dark + light) | phases, live log, usage, context tab with rule versions | L |
| E4-S3 | Task Detail drawer | AC checklist, related, AiPlan panel (approve/edit/reject), activity, the CTA "Run development agent" with a reason when it is disabled | M |

## Slice gate — end of week 8 (decision: continue or pivot)

The first 8 weeks are a vertical slice: M0 + M1 + the minimum of M2/M3. Before we invest
another 6 weeks, we measure on **one real repo, 10 tasks**:

| Metric | Threshold to continue | If it fails |
|---|---|---|
| PR merged without major rework | ≥ 60% | the agent loop is not sufficient → change the model/prompts, not the architecture |
| Human minutes per task | ≤ 40 min | if > 60 min, the platform does not save time → pivot to a "control layer" without its own agent |
| Cost per task | ≤ $5 | provider pricing or per-phase budget |
| "Would I miss it if I turned it off?" | yes | if not — stop and do not build M4/M5 |

The decision is recorded as an ADR (`docs/adr/`) with numbers, not as a feeling.

## M2 · Quality and git

**Exit:** the agent opens a PR, checks are returned to the platform, the review agent produces
findings, merge closes the task. No manual git work.

| ID | Story | AC | Dep |
|---|---|---|---|
| E5-S1 | `git.Provider` port + GitHub App + GitLab token + repo connect | one interface, two adapters; repo list; webhook verification per provider (HMAC / shared token) | L |
| E5-S2 | Branch + PR/MR creation (both providers) | branch `aip/{run}`; GitHub PR and GitLab MR; body = plan + test report + link to the run | M |
| E5-S3 | Check runs sync | `check_run` webhook → `test_results`; status in the UI | S |
| E5-S4 | Review agent | findings with severity; `blocker` blocks approval | M |
| E5-S5 | PR approval + merge gate | merge only through branch protection; `task.done` on merge | M |
| E5-S6 | Repositories + PR detail screens | checks panel (passed/running/pending), files with +/−, review findings by severity, "Merge" disabled until the checks are green | M |
| E6-S1 | Backlog board + task drawer | drag & drop changes status through the `transitions` API (optimistic + rollback) | M |
| E6-S2 | Task detail: Tests tab | shows the latest report + coverage delta | S |

## M3 · Blueprint and rules

**Exit:** a new project is created from a blueprint through the wizard; rules are versioned
and visible in the run audit.

| ID | Story | AC | Dep |
|---|---|---|---|
| E7-S0 | Blueprint JSON Schema + validation + instantiation | `spec` passes JSON Schema; `POST /projects` creates rules/tool_policy/test_gates/envs from the spec; sha256 is stored | M |
| E7-S1 | Blueprint model + versions | publish requires `approval.type=rule_publish`; immutable versions | M |
| E7-S2 | Template Builder wizard (5 steps) | validation per step; the draft is saved; "Finish" creates the project | L |
| E7-S3 | Rules editor + version history | diff between versions; publish changes `active_version` | M |
| E7-S4 | Context tab shows the exact versions | screenshot test: the run shows `org v4 / project v2 / module v1` | S |
| E7-S5 | Design System + Components registries | tokens from JSON; component preview; a11y tab | M |

## M4 · Delivery and analytics

**Exit:** deploy to staging through approval; prod requires two admins; analytics shows
cost and quality per project.

| ID | Story | AC | Dep |
|---|---|---|---|
| E8-S1 | Environments + deployments | deploy record with a log; rollback = a new deploy | M |
| E8-S2 | Prod approval (four eyes) | two different `user_id`s, both `admin`+; otherwise `403` | S |
| E9-S1 | Usage ledger + budgets | an overrun stops the run before the next LLM call | M |
| E9-S2 | Analytics summary + `source` | every KPI carries `measured`/`estimated`/`na` | M |
| E9-S3 | Settings API: members, providers, audit | audit filters; a provider secret is never returned in the API (`has_key` instead of the value) | M |
| E9-S4 | Settings screens (Members / Providers / Rules / Audit) | invite modal; capability matrix; rules editor + versions; audit table with filters | M |

## M5 · Hardening

**Exit:** load test of 50 parallel runs without API degradation; the security checklist
passed; documentation complete; runbook for incidents.

| ID | Story | AC | Dep |
|---|---|---|---|
| E10-S1 | Rate limits + concurrency limits | 429 with `Retry-After`; the per-org limit is respected | S |
| E10-S2 | Retention jobs | archive `run_events` > 90 days to S3; session deletion | S |
| E10-S3 | Backup/restore drill | RPO 15 min, RTO 1h; restore test in CI (nightly) | M |
| E10-S4 | Load test (k6) | 50 runs, p95 API < 300 ms, no OOM | M |
| E10-S5 | Security review + threat model | checklist from [07-testing.md](07-testing.md#7-security-checklist-before-every-production-deploy) | M |

## Definition of Done

A story is `done` only if **all 8** hold:

1. Code + tests; `make check` green (fmt, vet, lint, unit, integration, tsc).
2. The new endpoint is in `openapi.yaml` and `openapi-diff` is green.
3. The migration (if any) is forward-only and tested on an empty database **and** on a copy.
4. The UI has loading/empty/error states and works at 375px.
5. An audit event is written for every state change.
6. Metric/cost do not lie: `source` is accurate.
7. Documentation updated (if the contract changed) — in the same PR.
8. The PR description contains: what, why, risk, rollback, screenshot (for UI).
9. If the story touches the API, auth, or notifications — verified that no
   seam from [roadmap-later.md §2](roadmap-later.md#2-seams-the-mvp-must-preserve) is broken (S1–S12).

## Dependencies between epics

```
E1 ─┬─▶ E2 ─┐
    └─▶ E3 ─┴─▶ E4 ─▶ E5 ─▶ E6
                        └─▶ E7 ─▶ E8 ─▶ E9 ─▶ E10
```

Critical path: **E3 (agent loop)**. If E3 drags on, E4 and E5 do not move — better
to cut scope from E7/E9 than to let the agent loop be "almost done" in M3.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LLM quality insufficient for the plan/diff | medium | high | plan approval + iterative diff + replay harness with fixed inputs |
| Sandbox escape (the agent gets out) | low | critical | no docker socket, NetworkPolicy, read-only rootfs, egress proxy, "escape suite" test |
| Run cost explodes | medium | medium | hard budget before the call, per-org cap, alert at 80% |
| Scope creep in the MVP | high | high | this document is frozen; changes go through an ADR |
| Flaky tests block G2 | medium | medium | quarantine list, retry only for `known_flaky`, never globally |
