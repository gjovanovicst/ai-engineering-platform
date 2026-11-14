# 20 · Completion checklist

The single source of truth for "do we have everything". One row per story from
[05-mvp-plan](05-mvp-plan.md), with the evidence in the repository and the gap
that remains.

**Status:** ✅ done · 🟡 partial (works, but an AC is unmet) · ⬜ missing.
Updated: 2026-10-23.

Totals: **50 done · 0 partial · 0 missing** (50 stories, plus 3 items outside the original plan).

A story is only ✅ when its acceptance criteria and the DoD hold; "the endpoint
exists" is not enough.

## E1 · Foundation

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E1-S1 | Repo skeleton + Makefile + CI | ✅ | `Makefile`, `.github/workflows/ci.yml`, `scripts/e2e.sh` | — |
| E1-S2 | Config + slog + OTel bootstrap | ✅ | `internal/config`, `internal/platform/log`, `internal/platform/telemetry` | — |
| E1-S3 | Postgres + goose migrations + RLS | ✅ | migrations 0001–0021, `db.WithOrg` | — |
| E1-S4 | Domain: org/user/project/task | ✅ | `internal/domain` (+ run/repo/blueprint/rules types) | — |
| E1-S5 | Auth: email+password, JWT, refresh, CSRF | ✅ | login/refresh rotation/reset/OIDC; account menu signs out the current session or **every** session (`POST /auth/logout-all` revokes all refresh tokens) | web sessions are bearer, not cookie+CSRF yet |
| E1-S6 | RBAC middleware | ✅ | `roleAtLeast` in every handler | no central policy file; checks are per endpoint |
| E1-S7 | CRUD org/project/task + OpenAPI handlers | ✅ | projects/tasks full; org GET/PATCH/DELETE (`/orgs/{org}`, owner, slug confirmation) plus the Settings → Workspace tab | `max_concurrent_runs` is not editable from the UI |

## E2 · Web shell

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E2-S1 | AppShell + tokens + theme | ✅ | `app-shell.tsx`, `tokens.json`, `globals.css`; light/dark toggle with localStorage persistence, account menu (Settings, Overview, Sign out) | — |
| E2-S2 | Sign-in + project list + task list | ✅ | `/login`, `/projects`, project detail | — |
| — | Onboarding overview (not in the original plan) | ✅ | `/overview`: the loop, live setup checklist, glossary; login and `/` land there | new page; add it to the plan if kept |
| — | Notifications (in-app + email, not in the original plan) | ✅ | migration 0026, bell with unread count, plan/PR/failure notifications to admins, mark read/all; SMTP email through Mailhog for each admin | — |
| — | Gitea/Forgejo + Bitbucket adapters (E11-S2, proposed P2) | ✅ | Gitea: REST create/merge + `X-Gitea-Signature`; Bitbucket: REST create/merge + `X-Hub-Signature` (Basic auth); both in the provider picker, clone bases and config | not run against live hosts |
| — | GitHub App webhook (P3) | ✅ | `POST /webhooks/github-app` verifies the app secret, resolves the repo by `repository.full_name` and reuses check/merge handling; worker uses the app installation token for checks when configured; app creation itself is a one-time GitHub UI step (docs/27) | app credentials are env config, not per-org DB |
| — | CI checks sync (P3) | ✅ | worker polls open PRs every minute; GitHub check-runs, with an Actions-jobs fallback for tokens without `checks:read`; recorded checks unblock the merge gate even when webhooks are down | webhook remains preferred (lower latency) |
| — | Project E2E command (P3) | ✅ | `projects.e2e_command`, run in a sandbox after unit tests, `e2e` check blocks the PR; set in Delivery → Checks | BYO visual diff command; built-in baselines pending |
| — | MCP stdio client (P3) | ✅ | initialize/tools-list/tools-call over newline JSON-RPC, fake-server test; not yet wired into the agent tool loop | env + project config, sandbox start, per-run lifecycle in both backends, `mcp.unavailable`; UI field | sandbox fake-server test/metrics pending |
| — | Component scaffolding (P4) | ✅ | scaffold endpoint creates a spec task; UI in Design; live T-29 | token codegen direct file generation pending |
| — | CLI companion (P4) | ✅ | `aipctl tasks/start/run/approve` against the API | IDE/plugin SDK pending |
| — | Operations runbook + SLOs (P4) | ✅ | docs/33 deploy/upgrade/rollback/incidents + provisional SLOs | HA/failover, alert routing, trace-link pending |
| — | Repository SBOM (P4) | ✅ | CycloneDX job on push/PR with artifact upload | image cosign + provenance/SBOM attestations in CI | Go license allowlist in CI; deploy-time verification pending |
| — | Audit hash chain (P3) | ✅ | trigger + backfill + `aip_audit_broken_count()`; insert tested transactionally | full-rewrite detection needs an external witness |
| — | Policy dry-run (P4) | ✅ | `POST /projects/{id}/policy/evaluate`; fixed persistence of forbidden paths | open-PR dry-run via GitHub/GitLab diff fetch |
| — | Task dependencies (P3) | ✅ | `depends_on` (create-only, max 20), run start blocked until deps are done; cycle-free by construction | editing/cycle detection + UI pending |
| — | Threat model + audit design (P3) | ✅ | docs/31 controls/residuals, docs/32 hash-chain vs witness options | hardening items T1/T2/T8 remain |
| — | Worker/queue metrics (P3) | ✅ | worker `/metrics` on `:9090` with queue depth/oldest job and `aip_runs_total` by status via a security-definer aggregate | alerts pending |
| — | HTTP metrics (P3) | ✅ | `/metrics` exposes `aip_build_info`, request counters and duration totals by method/status; `/metrics` itself is excluded | SLO dashboards/alerts pending |
| — | Backup + restore drill (P3) | ✅ | custom-format dump + SHA-256 + retention; restore into scratch DB verified (12 orgs, 373 runs, 39 migrations); docs/29 | schedule/off-host storage pending |
| — | Secret scan on diffs (P3) | ✅ | AWS/GitHub/Slack/private-key/generic patterns on added lines; placeholder allowlist; `secrets` check fails the run with `secret_detected` | — |
| — | Repository map (P3) | ✅ | relevance-ranked file map appended to the agent prompt, recorded as `context.repo_map` | AST/MCP/adapter pending |
| — | Project preflight (P3) | ✅ | readiness checks for test command, repo, design standard, required checks, e2e, agent backend and GitHub App; shown in Delivery → Checks with fix hints | manifest flow creates and stores the app; per-org webhook route; installation id manual |
| — | Task attachments (P3) | ✅ | upload/list/download per task (5 MiB; png/jpeg/webp/pdf/txt), SHA-256, RLS-scoped; metadata reaches the agent prompt and the context manifest | no malware scan/S3 yet |
| — | Context manifest v2 (P3) | ✅ | run manifest lists task/rules/design/component/repository sources with versions and hashes, precedence and a deterministic hash; run page renders them | attachments, freeze/delta after approval pending |
| — | Acceptance proof contract (P3) | ✅ | `AIP_PROOF` maps every structured AC to a test in the diff or a manual reason; proof is uploaded and run-bound; missing/invalid proof fails `acceptance_unproven`; PR body renders the criterion → evidence table; the new test is replayed on the base archive in a sandbox and must fail (`acceptance.base_verified`); manual evidence requires a reviewer decision note and records `acceptance.manual_ack` | — |
| — | Acceptance verifier (P3) | ✅ | development runs record an `acceptance` check; source changes without a test file fail the run with `acceptance_missing` before a PR is opened | templates/styles/docs are exempt |
| — | Designer workflow (P3) | ✅ | `designer` role (writes design standards/components, cannot manage members), structured standard builder (a11y/responsive/patterns + typography/spacing/radius JSON), component preview with example + copy | — |
| — | Component allowlist + import compliance (E12-S3/S5, proposed P3) | ✅ | `project_components` allowlist with allow/exception modes and a required reason for exceptions; agents receive the allowlist; deprecated/forbidden import paths fail the `design` check | visual regressions still manual |
| — | Developer Tools navigation (E12-S6, proposed P3) | ✅ | Design standards + Component registry grouped under DEVELOPER TOOLS, visible read-only to every member (writes stay admin/owner); project page keeps the standard selector; shell restores claims after a refresh | — |
| — | Component Registry 2.0 (E12-S2, proposed P3) | ✅ | `component_entries` gains category/framework/package/import_path/version/docs/example and approved/forbidden statuses; registry UI shows metadata and edits all fields; update requires the name (fixed a latent 400) | project component allowlist is still pending |
| — | Design compliance (E12-S5, proposed P3) | ✅ | development runs scan the diff for raw hex colours (token `var(--…)` allowed), record a `design` test result and a `design.compliance` event; require `design` in project checks to gate merges | component/import rules await E12-S2 |
| — | Design context for agents (E12-S4, proposed P3) | ✅ | worker appends the project's published standard (tokens, a11y, responsive, patterns) to the prompt, emits `design.snapshot`, and the context manifest carries `design: <id>@<version>` | component allowlist is E12-S2 |
| — | Project design standards (E12-S3, proposed P3) | ✅ | `projects.design_system_id` FK + version; `GET/PUT /projects/{id}/standards` (only published versions, draft → 400); project page Design standard selector | agent context is E12-S4; component allowlist is E12-S2/S3b |
| — | Design standards v1 (E12-S1, proposed P3) | ✅ | `design_standards` with versions, publish/immutability trigger, audit, API + Design System panel (create from tokens, new version, publish, token diff) | project assignment/agent context is E12-S3/S4 |
| — | PWA (E11-S1, proposed P2) | ✅ | manifest, SVG icon, production-only service worker, `/offline` shell; dev never registers the worker | not yet install-tested on a phone |
| — | Redis SSE fan-out (not in the original plan) | ✅ | every `run_events` insert publishes to `run:{id}`; SSE backfills by seq and then wakes on pub/sub; DB polling remains the fallback | no per-replica sharding needed |
| E2-S3 | Primitives: EmptyState, Alert, Modal, Toast, Skeleton | ✅ | `components/ui`: EmptyState, Alert (4 variants), Modal (Escape, aria-modal), ToastProvider/useToast (auto-dismiss) and Skeleton/SkeletonText; wired in Providers | not yet adopted everywhere |

## E3 · Agent loop

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E3-S0 | Agent runtime spec → code | ✅ | `internal/agent` | — |
| E3-S0a | `AgentBackend` adapter | ✅ | `CommandBackend`, `HTTPBackend`, per-run selector | — |
| E3-S1 | runs + state machine + run_events | ✅ | migration 0007, `internal/app/runs` | — |
| E3-S2 | River `agent_run.execute` + retry | ✅ | `internal/platform/queue`, `cmd/aip-worker` | — |
| E3-S3 | Provider adapters (openai-compatible, anthropic) | ✅ | OpenAI-compatible HTTP backend; Anthropic Messages API with the same sandbox tool loop; fake-server test | — |
| E3-S4 | Context assembly + `rules_snapshot` | ✅ | task briefing + approved plan + published rules; deterministic manifest (`sha256` over sorted repo files + rule refs) emitted as `context.manifest` and stored as the `context_manifest` artifact | prompt assembly is still text concatenation |
| E3-S5 | Planning agent + plan artifact | ✅ | `runs.Runner`, `AIP_PLAN`, plan JSON + sha256 | — |
| E3-S6 | Approval flow (request/decide/expire) | ✅ | plan/pr approvals, decisions and gates; pending approvals expire lazily and via a worker sweep every minute (event `approval.expired`) | expired plan approvals leave the run parked; a follow-up story can fail them |
| E3-S7 | Sandbox adapter | ✅ | Docker + Kubernetes Job drivers, image, network policy | K8s path tested with a fake clientset only |
| E3-S8 | Development agent + diff + commit | ✅ | host-side `git.Cloner` (depth 1, base branch) → tar → sandbox; diff against the clone; push keeps history | shallow clone only; no submodules/LFS |
| E3-S9 | Test phase + `test_results` | ✅ | `AIP_TEST_COMMAND` executed by the tool loop, output + status stored in `test_results`, gate fails on non-zero; Tests tab shows it; plan/diff/review/test output also uploaded to S3 when configured (`run_artifacts`) | — |
| E3-S10 | SSE endpoint + backfill | ✅ | `/runs/{run}/events`, `Last-Event-ID`, Redis pub/sub fan-out with DB polling as the fallback | — |

## E4 · Screens

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E4-S1 | Dashboard (light) | ✅ | `/analytics` | — |
| E4-S2 | Agent Run Details | ✅ | `/projects/:id/runs/:id` (log, plan, diff, review) | — |
| E4-S3 | Task Detail drawer | ✅ | `task-drawer.tsx` (Escape, focus trap) | read-only; no edit/assign |

## E5 · Git

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E5-S1 | git.Provider + GitHub App + GitLab token + repo connect | ✅ | both adapters; GitHub App installation tokens (RS256 JWT -> `/app/installations/{id}/access_tokens`, cached; used for clone, push and merge) with `AIP_GITHUB_APP_*`; static token remains the fallback; GitLab token | App path is unit-tested against a fake API but not yet run against a live GitHub App |
| E5-S2 | Branch + PR/MR creation | ✅ | `git.Publisher`, `HostPusher`, both providers; the PR title is `T-N: task` and the body carries What/Why/Plan/How tested/Risk/rollback plus a run link | body is generated, not editable per project |
| E5-S3 | Check runs sync | ✅ | migration 0023 `test_results`, GitHub `check_run` + GitLab pipeline webhooks, `GET /pull-requests/{id}/checks`, badges, required checks per project (migration 0024) and a gate that blocks failed/missing/pending required checks | no annotations or retry policies |
| E5-S4 | Review agent | ✅ | `AIP_REVIEW`, findings stored + UI; a `blocker` finding blocks PR approval (checked before the decision is recorded) | no auto-fix loop |
| E5-S5 | PR approval + merge gate | ✅ | approvals type `pr`, `ApprovedForPR`, auto-merge | — |
| E5-S6 | Repositories + PR detail screens | 🟡 | repo panel/list, PR list, run diff/findings | no PR detail page with checks/files/review |

## E6 · Backlog

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E6-S1 | Backlog board + task drawer | ✅ | board with six columns, drag & drop through the transitions API (optimistic + rollback), drawer edits title/description/priority (PATCH) | — |
| E6-S2 | Task detail: Tests tab | ✅ | task drawer Tests section (status + output of the latest run) | — |

## E7 · Blueprint, rules, design

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E7-S0 | Blueprint schema + validation + instantiation | ✅ | migration 0013, `internal/app/blueprints`, starter tasks | — |
| E7-S1 | Blueprint model + versions | 🟡 | publish/immutability trigger | **single version**; no new version/change request |
| E7-S2 | Template Builder wizard (5 steps) | ✅ | `/blueprints` wizard | — |
| E7-S3 | Rules editor + version history | ✅ | `/rules`, draft/publish, sha256; line diff between any two versions; publishing requires a second admin (`rule_publish`, migration 0022) | — |
| E7-S4 | Context tab shows the exact versions | 🟡 | `rules.snapshot` event with rule refs | **no UI tab**; only the event payload |
| E7-S5 | Design System + Components registries | ✅ | migration 0021, `/design-system` and `/components` are editable (CRUD + seed from the built-in baseline) | no per-entry version history or component preview |

## E8 · Environments

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E8-S1 | Environments + deployments | ✅ | migration 0017, project panel, deploy records | no log streaming, no rollback action |
| E8-S2 | Prod approval (four eyes) | ✅ | approver ≠ requester, admin only | — |

## E9 · Usage and settings

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E9-S1 | Usage ledger + budgets | ✅ | migration 0019, cost per model; run budget enforced per step (`budget_exceeded`, partial usage); run page shows tokens, cost and budget | command/streaming backends clamp wall clock only |
| E9-S2 | Analytics summary + `source` | ✅ | `/metrics`, `docs/16-metrics.md` | — |
| E9-S3 | Settings API: members, providers, audit | ✅ | `/members`, `/providers` (encrypted), `/audit` | — |
| E9-S4 | Settings screens | ✅ | Members/Invites/Rules/Providers/Audit tabs | — |

## E10 · Hardening

| ID | Story | Status | Evidence | Gap |
|---|---|---|---|---|
| E10-S1 | Rate limits + concurrency limits | ✅ | login/register/reset limiters; per-org 600 req/min with 429 + `Retry-After`; max 5 active runs per org (429) | limits are in-memory per API replica |
| E10-S2 | Retention jobs | ✅ | daily worker pass (`aipctl retention` too): expired sessions/invites/resets/verifications purged; `run_events` > 90 days archived as per-run gzip NDJSON in S3/MinIO | events are archived but not deleted (append-only rule) |
| E10-S3 | Backup/restore drill | ✅ | `scripts/backup.sh`, `restore-check.sh`, CronJob | — |
| E10-S4 | Load test (k6) | ✅ | `scripts/k6/loadtest.js` + `make loadtest-k6`: 50 runs, p95 **118.67 ms**, 0 failed requests | memory is observed via container limits, not asserted |
| E10-S5 | Security review + threat model | ✅ | checklist run in `docs/15` §10: sandbox hardening verified, webhook dedupe, govulncheck 0 code vulns, pnpm audit clean | internal review only; no external pentest |

## Residual risks (accepted, not stories)

| Risk | State | Mitigation / next step |
|---|---|---|
| **GitHub App not exercised live** | unit-tested against a fake API; PAT fallback works | enter `AIP_GITHUB_APP_*` and run one PR end to end |
| **GitLab / OIDC / OIDC providers are adapter-tested, not live-tested** | fakes + docs | verify with a real GitLab project and OIDC tenant |
| **Web auth is bearer, not cookie+CSRF** | documented in `docs/15` | BFF cookie work would add double-submit CSRF |
| **In-memory limits and webhook dedupe** | per API replica | move to Redis counters / a deliveries table |
| **Notifications: in-app + SMTP via Mailhog** | production SMTP relay needed | configure `AIP_SMTP_ADDR` with TLS |
| **No external security review** | internal checklist passed (`docs/15` §10) | book a pentest before a public pilot |
| **Retention archives events but does not delete them** | append-only rule | operator-run purge after a retention decision |
| **P2 items** (mobile/PWA, Bitbucket/Gitea, billing, SSO mapping) | out of the M0–M5 plan | new cycle |

All 50 stories are done; items outside the original plan (Overview, Notifications,
Redis SSE fan-out) are listed in the table above. See
[24-final-report.md](24-final-report.md) for the delivery summary and evidence.
