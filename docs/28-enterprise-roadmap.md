# Enterprise readiness roadmap

Status: **in progress (rev 3)** · 2026-10-27 · owner: platform
Implemented slices: A1 proof/base guard, A2 retry/flaky, A3 manifest sources +
attachments, A4 e2e + BYO visual command, A5 preflight + GitHub App manifest/app
tokens, A6 repo map/symbols + MCP (stdio, namespacing, sandbox lifecycle, project
config, call events), B secret scan + HTTP metrics + backup drill.
Scope: what the AI Engineering Platform needs before it can be trusted to build
small, medium and large enterprise software — and in which order.

This document is a plan, not a promise. Estimates assume four people (2 backend,
1 frontend, 1 platform/DevOps), are **not additive** (phases overlap) and must be
re-checked after each phase. Items are ordered by risk reduction per unit of
work, not by novelty.

Revision 2 adopts the review feedback: proof must be bound to a commit and a test
version, context is an explicit versioned artefact, parallel execution must be
safe before task decomposition, and security/operations cannot wait for the last
phase.

## 1. Where the platform stands

What already works and must not be rebuilt:

- task → plan approval → development → test gate → design check → PR → CI →
  human approval → merge, with an append-only run timeline
- Postgres row-level security per organisation, encrypted provider secrets,
  Argon2id passwords, EdDSA sessions
- four-eyes approvals, approval expiry sweeps, required checks and a merge gate
- versioned design standards, component registry, project allowlist/exceptions,
  agent context injection and a deterministic design check
- CI check ingestion plus a polling fallback (`checksync`) when webhooks fail
- GitHub App webhook route (verified with a signed request), per-repo hooks with
  a PAT fallback, GitHub/GitLab/Gitea/Bitbucket publishing
- artifacts, notifications, usage ledger, per-run budgets
- `context.manifest` per run: task, agent type, model, files, rule versions,
  design-standard id/version and a hash (partial — see §4 A3)

Known weaknesses, observed in real runs (not hypothetical):

| Observation | Consequence |
|---|---|
| T-21 added `pagination.go` but never wired the handler/template; CI passed | "done" can mean "compiled" |
| planning occasionally returns no `AIP_PLAN` | tasks stall with `plan_missing` |
| an agent wrote a temporary failing test to inspect a diff | the test gate aborts a valid run |
| acceptance only checks that *a* test file changed | no proof the acceptance criteria are covered |
| a test can exist and pass without exercising the behaviour | evidence can be theatre |
| PAT lacks `checks:read` / `admin:repo_hook` | onboarding and CI depend on manual scopes |
| one worker, sequential runs | throughput limited; no parallel task graph |
| no merge queue | concurrent PRs require manual rebases |
| no preview environment | reviewers cannot see UI before merge |
| no attachments or wireframes | UI tasks rely on links in the description |
| no project-level RBAC | org roles grant access to every project |
| audit is append-only by application policy, not tamper-evident | DB admins can rewrite history |

## 2. Positioning

The platform is **not** a better IDE agent and should not try to be one.
It is the **governed execution layer** around agents (Claude Code, DeepSeek,
in-house models): it provides context, policy, verification, approvals, audit and
delivery. "Bring your own agent" is a feature, not a compromise.

North star: *every change is reproducible, proven and approved.*

A useful distinction for the roadmap: features are what the platform has;
trust is what it can **demonstrate**. Every item below therefore has an
observable acceptance test, not just a capability.

## 3. Evidence model (cross-cutting)

Acceptance evidence has three levels:

| Level | Meaning | Who can grant it |
|---|---|---|
| `automated` | a named test passed at this commit and is mapped to the criterion | the platform |
| `manual` | an authorised reviewer verified the behaviour and recorded a reason | a human, audited |
| `unverified` | no acceptable evidence | nobody — blocks review |

An automated proof records: criterion id, test id/name, command, result, commit
SHA, hash of the test file(s), evidence hash, timestamp and the run id. It is
invalid when the commit changes or the referenced test changes.

Pragmatic guard against "a test that always passes": the AC test must fail on the
**base commit** (without the change) and pass on the head commit. When that is not
practical (infra-dependent, flaky setup), the proof is downgraded to `manual`
with an explicit reason. Mutation testing is an optional P2 tool for critical
modules, not a P0 requirement.

## 4. Phase A — provable delivery (target: 8–10 weeks)

Exit criterion: a task cannot reach `review` without commit-bound evidence for
every structured acceptance criterion, and every run carries a versioned context
manifest.

### A0. Information architecture (done 2026-10-25)

Project pages are split into Overview / Board / Design / Delivery, design pages
into Standards / Tokens / Baseline, component pages into Registry / Primitives;
forms open in a portalled side panel with footer actions. This is the baseline
for the empty/loading/error states later stories add.

### A1. Proof contract and acceptance verifier (critical) — implemented 2026-10-25 (first slice)

Why: the largest quality gap. File-presence checks do not prove behaviour.

Scope:
- structured acceptance criteria get stable ids; the development agent returns
  `AIP_PROOF` mapping each criterion to one or more evidence entries
- the runner validates: the test exists, ran in this run, passed, and its file is
  part of the diff (new or changed)
- negative guard: run the referenced tests against the base SHA and require
  failure (configurable per project; otherwise downgrade to `manual`)
- proof is stored as an artifact and attached to the criterion, bound to the
  commit and test hash; a new commit invalidates it
- `manual` evidence requires a named reviewer, a reason and an audit event
- a missing/unknown/invalid proof fails the run with `acceptance_unproven`
- the PR body renders the criterion → evidence table

Out of scope: natural-language theorem proving; mutation testing.

Acceptance:
- removing the test that proves an AC fails the run
- a test that passes on both base and head cannot be `automated` evidence
- changing the code after approval invalidates the old evidence
- three ACs cannot pass with one test unless the proof maps all three

Estimate: 4–5 weeks.

Implemented: `AIP_PROOF` parsing, per-criterion mapping, test-must-be-in-diff validation, manual evidence with a reason, proof artifact, commit/run binding, `acceptance_unproven` failure, agent retry once when proof is missing, and a base-SHA negative guard (the acceptance tests are replayed on the base archive with the new test files in a sandbox; a passing test invalidates the proof) and a criterion → evidence table in the PR body. Manual evidence now requires a decision note naming the reviewer; the approval records an `acceptance.manual_ack` event with reviewer, reason and criteria.

### A2. Agent reliability quick wins (start in week 1) — implemented 2026-10-25

Why: `plan_missing`, self-inflicted test failures and retry costs waste runs and
pollute the metrics used by A1.

Scope:
- structured-output retry: when `AIP_PLAN`/`AIP_PROOF` is absent, re-prompt once
  with the parser error, under an explicit retry budget (attempts + tokens)
- every retry records a reason and is visible in the run timeline
- classify failures as `infra`, `flaky` or `code`; only `code` fails the task
  gate; classification is conservative (unknown → `code`), `infra` retries once
- temporary test-file names (`tmp_*`, `zz_*`) are a **warning**, not an automatic
  rejection; the failure reason is surfaced
- surface the classification and failing test in the UI

Acceptance:
- a planning run without the sentinel retries once and either succeeds or fails
  with a readable reason and the retry count
- a network failure during `go test` does not mark the task failed without an
  `infra` classification
- retries stop at the configured budget

Estimate: 2–3 weeks (parallel with A1).

Implemented: one structured-output retry under an output-token budget, retry reason events, conservative `infra`/`flaky`/`code` classification (`test_infra`, recovered flaky runs emit `test.flaky_recovered`), temporary test files as a warning. A run-view Retry panel lists `run.retry` and `test.flaky_recovered` with reason/attempt.

### A3.

### A3. Context manifest v2 and attachments (high) — implemented 2026-10-25 (first slice)

Why: reviewers must be able to answer "why did the agent implement it this way?".
The current manifest is partial: no attachments, no precedence, no change log.

Sources to version per run:

| Source | What it contributes |
|---|---|
| Task | request, acceptance criteria, priority |
| Attachments | screenshots, wireframes, PDFs, specifications |
| Project standards | architecture, coding rules, constraints |
| Design standard | tokens, themes, UX rules |
| Component registry | approved libraries/components |
| Repository snapshot | existing code and patterns |
| API documentation | contracts and integrations |
| Previous runs | relevant failures and lessons |

Rules:
- precedence when sources conflict: **org policy → project standards → design
  standard → task references → repository conventions → model defaults**
- every source records id, version, checksum and visibility
- the manifest is frozen when the plan is approved; a later change is recorded as
  a delta and requires re-approval when it affects the plan
- the UI shows the manifest and the deltas, not just a hash
- attachments need their own security story: storage, size/type limits, malware
  scanning, access control, retention and signed URLs

Out of scope: image understanding guarantees; automatic wireframe-to-code.

Acceptance:
- every run lists the sources and versions the agent received
- changing a design standard version after plan approval is flagged
- an attachment is available to the agent and to the reviewer, with an audited
  download

Estimate: 4–5 weeks (attachments are a separate story with security review).

Implemented first slice: manifest lists versioned sources (task, rules, design standard, component registry, attachment, repository snapshot), a precedence order, a timestamp and a deterministic hash; the run page renders the sources and precedence. Task attachments (png/jpeg/webp/pdf/txt, 5 MiB) are stored with a hash, listed/downloaded in the task drawer and included as a manifest source; agents receive attachment metadata only. Plan approval now freezes the planning run manifest hash into the development run; a changed manifest emits `context.changed` (frozen/current hashes) and fails `context_changed`, requiring a new approval. `context.changed` also lists the changed sources (`changed_sources`). Attachments are scanned through clamd (zINSTREAM) when `AIP_CLAMAV_ADDR` is set; with `AIP_S3_ENDPOINT` the bytes go to S3/MinIO and Postgres keeps metadata + `object_key` (bytea fallback when S3 is off).

### A4. End-to-end and visual evidence (high) — e2e command implemented 2026-10-25

Why: UI tasks pass with unit tests only; design regressions are invisible.

Scope:
- project-level `e2e_command` (Playwright/Cypress) in the sandbox after unit
  tests; trace + screenshots stored as artifacts
- screenshot comparison when a design standard is assigned: stable environment,
  masked dynamic content, configurable thresholds, stored baselines, reviewer can
  accept a new baseline
- separate `e2e` and `visual` checks; both can be required for merge
- visual failures are a signal with an explicit override + reason, not a silent
  block

Acceptance:
- a broken navigation flow fails `e2e` before the PR opens
- a colour change that violates the standard fails `visual` with a diff image
- a reviewer can accept a new baseline, audited

Estimate: 4–5 weeks (overlaps A1/A3).

Implemented: project-level `e2e_command` and `visual_command` run in a fresh sandbox after the unit tests, record blocking `e2e`/`visual` checks. Visual is BYO-tool (Playwright/Percy): the platform enforces the command, not a built-in pixel comparator. A built-in PNG comparator (`POST /projects/{id}/visual-diff`, masks + threshold) and S3 baseline storage (`PUT /projects/{id}/visual-baseline`, compare by `name`) exist; Screenshots written under `visual/*.png` by the visual command are compared with the stored baselines and fail the run on a mismatch (`visual.baseline_diff`/`visual_failed`); missing baselines are recorded and skipped. Per-project threshold and masks are stored (`PUT /projects/{id}/visual-policy`) and used by the run gate. Reviewers can open a stored baseline from Delivery → Checks (`GET /projects/{id}/visual-baseline`). Side-by-side baseline/current images are in the run view; each captured screenshot is stored as a `visual/<name>` run artifact and served by `GET /runs/{run}/visual?name=...`.

### A5. Onboarding and preflight (high) — preflight implemented 2026-10-25

Why: this session needed a tunnel, a PAT with two scopes, a manual webhook edit
and a manual check injection. That is not a customer experience.

Scope:
- GitHub App **manifest flow**: create from Settings, callback stores app id,
  key and webhook secret encrypted per organisation
- webhook URL/events configured automatically; installation id stored
- preflight before the first run: provider/repo access, check permissions,
  sandbox availability, test command, design standard and agent/model config
- actionable error messages with a "fix" link, not a stack trace
- measure onboarding from **start to first governed PR**, including waits

Acceptance:
- a fresh repository reaches its first governed PR in under 10 minutes without
  editing `.env` or the GitHub UI
- a revoked installation or missing check permission is reported before a run
  starts

Estimate: 3–4 weeks.

Implemented: `GET /projects/{id}/preflight` reports test command, repository, design standard, required checks, e2e command, agent backend and GitHub App status with a fix hint (Delivery → Checks). The GitHub App manifest flow creates the app from Settings, stores the encrypted key/webhook secret per organisation, exposes a per-org webhook route and records the installation id manually. Per-org installation tokens now feed clone, push, PR creation and checks polling (fallback to the static PAT if the app is missing/uninstalled). Installation is automatic: the manifest includes a one-time `setup_url` state and GitHub redirects the installation id back. Branch protection can be applied from Delivery → Checks (`POST /projects/{id}/protection`) using required checks + one review. Onboarding time (project creation → first merged PR) is measured in Analytics. Per-project onboarding steps are shown in Analytics (`task_hours`, `run_hours`, `pr_hours`, `onboard_hours`) and can be expanded per project.

### A6. Agent capability and repository context (high, starts in A) — repo map implemented 2026-10-25

Why: proof cannot compensate for an agent that cannot find the relevant code.
This is the main gap not covered by the review.

Scope:
- repository index with semantic search and AST-aware navigation (Go/TS first)
- MCP tool support for external context
- Claude Code / DeepSeek adapter as an alternative backend behind the same proof
  and policy gates
- model routing by task type and budget
- measure first-pass success and rework per backend

Acceptance:
- the same pagination task is completed with fewer missed edits than T-21
- switching backend does not change the approval or evidence contract

Estimate: 6–8 weeks (incremental; adapter first, index second).

Implemented: relevance-ranked repository map and Go/TS symbol hints in the
development prompt (`context.repo_map`, `context.symbols`); semantic ranking via
an OpenAI-compatible embeddings client with S3 vector cache (keyword fallback);
MCP over stdio with namespaced tools, per-server sandboxes (`allow_network`),
env/project config, preflight validation, `mcp.call` metrics and a tool view fed
by the latest run (`mcp.tools`); Claude Code adapter preset with an opt-in
sandbox image (CLI 2.1.311 verified). **Not yet:** live embedding-provider test
and an actual Claude Code agent run (only the CLI version was verified).

## 5. Phase B — reliable operations and security baseline (target: 3–4 weeks after A)

Exit criterion: onboarding is self-service, failures are diagnosable, and the
security baseline is documented and tested.

- threat model: sandbox escape, egress, secrets, untrusted code, supply chain
- sandbox hardening: read-only mounts, per-run resource caps, egress allowlist
  verification, no host docker socket
- secret scanning on diffs before push (implemented: `secrets` check, `secret_detected` failure); prevent credentials in artifacts/logs
- basic monitoring and alerting: API `/metrics` has request counters/durations; worker `:9090/metrics` has queue depth/oldest job and run counts by status; alert rules are in `deploy/observability/alerts.yml` (API/worker down, queue lag/depth, 5xx)
- documented backup/restore drill with measured RPO/RTO
- dependency/SAST scanning enabled as required checks for the platform itself
- audit integrity design (see D2) is written down before scale

Estimate: 3–4 weeks.

Implemented: diff secret scan, clamd upload scan, HTTP/worker/queue/run metrics,
alert rules, an executed backup/restore drill (docs/29), threat model (docs/31),
audit hash chain (docs/32, migration 0047), MCP output scanning and a CI
`security` job. **Not yet:** domain-level egress allowlist, seccomp/AppArmor,
external audit witness, and Prometheus/Alertmanager deployment wiring.

## 6. Phase C — controlled scale (target: 3–4 months)

Exit criterion: ten independent changes can run and merge concurrently without
duplicate work, lost updates or manual rebases.

Order is deliberate: parallel infrastructure before task decomposition.

1. **Worker pool and queue fairness** — multiple workers, per-org quotas, retry
   budgets, cancellation. Worker concurrency is configurable
   (`AIP_WORKER_MAX_WORKERS`, default 4); per-org concurrency is gated by
   Postgres advisory-lock slots (`AIP_ORG_MAX_CONCURRENT_RUNS`, default 2).
2. **Isolation and idempotency** — one worktree/sandbox per run, unique job keys,
   no double publish on retry.
3. **Merge queue** — rebase, re-run required checks, merge in order; conflict →
   back to the author agent with context.
4. **Task dependency graph** — subtasks with `depends_on`, cycle detection,
   bounded fan-out. PR strategy is chosen per graph and risk:
   - integrated parent PR,
   - independent PRs,
   - ordered stacked PRs.
   Default: independent for independent subtasks; stack for ordered work.
5. **Monorepo support** — affected-package detection (Go workspaces, pnpm/nx),
   per-package owners/checks, cache-aware tests.
6. **Preview environments** — ephemeral per PR, seeded, TTL, auto-destroy, link in
   the PR; screenshots feed A4.
7. **Provider parity** — GitLab/Gitea/Bitbucket check listing, branch protection,
   webhook parity tests.

Acceptance:
- 10 concurrent PRs merge without manual rebases and without duplicate PRs
  (service-level merge-queue test and the ten-PR API e2e in docs/57; fake GitLab,
  no real conflict/rebase)
- a conflicting merge returns actionable context to the agent
- a one-line monorepo change does not run unrelated package tests
- a reviewer opens the changed UI before merge

Estimate: 3–4 months (overlapping items).

## 7. Phase D — enterprise (target: 4–6 months)

Exit criterion: a security review can be answered from the product, not from
spreadsheets.

1. **Identity and access** — SAML/OIDC SSO, SCIM (demand-driven), **project-level
   RBAC**, service accounts, session policies (idle timeout, IP allowlist,
   re-auth for approvals).
2. **Audit, retention, residency** — tamper-evident audit (hash chain + signed
   export), SIEM webhook, retention/legal hold, deletion workflow, regional
   storage where required.
3. **Supply chain** — SAST, dependency/license scanning, secret scanning, SBOM,
   signed artifacts (cosign), provenance.
4. **Policy-as-code** — required checks, approval rules, allowed models/tools,
   change windows, max diff size, forbidden paths; dry-run against open PRs.
5. **Operations** — HA API/workers, queue fairness under load, tested backup/
   restore, upgrade/rollback runbook, SLOs and alerting, OTel traces linked from
   run pages (no prompt content in traces).
6. **Demand-driven only** — BYOK, full SCIM lifecycle, multi-region residency:
   implement when a named customer requires them; keep the architecture ready.

Acceptance:
- a project-level role cannot read another project's secrets, runs or approvals
- an audit export verifies against its hash chain
- restore from backup is demonstrated within the documented RTO
- a policy change can be dry-run before it blocks anyone

Estimate: 4–6 months.

## 8. Phase E — product depth (continuous)

Implemented first slice: `aipctl tasks|start|run|approve` uses the API
(`AIP_API_URL`, `AIP_ORG_ID`, `AIP_API_TOKEN`) for task listing, starting runs,
showing a run and recording approvals without the web UI.

- **Component scaffolding**: "Create component" task produces files, registry
  entry, tests and a preview from the design standard; token codegen for
  React/Vue/Angular.
- **Figma/Storybook import**: first slice: `POST /components/import` accepts Storybook/Figma JSON entries (created/skipped by import path; approved by default); component registry has an "Import Storybook JSON" file input. Figma import: `POST /components/figma` reads COMPONENT nodes (`AIP_FIGMA_TOKEN`, optional `AIP_FIGMA_API_URL`) and imports them via the registry; UI button in Component registry. Visual references remain.
- **CLI and IDE companion**: submit tasks, see runs/evidence, approve, without the
  web UI.
- **Plugin SDK**: first slice: project `plugin_checks` (max 5 named sandbox commands) run after e2e/visual, recorded as `plugin:<name>` checks and blocking with `plugin_failed`; UI JSON editor. Custom agents: [44-adr-custom-agents.md](44-adr-custom-agents.md) — registry, digest validation, project selection and worker execution are implemented and proven by the fake-GitLab e2e; a Settings UI registers/selects plugins; a digest allowlist and a fail-closed cosign verifier exist; the API image ships cosign and a live local-registry drill accepted a signed plugin (201) and rejected an unsigned one (400). Blueprint templates: three presets (SaaS monorepo, Go API, Next.js UI) prefill the wizard.
- **Delivery analytics**: DORA metrics, agent success rate, cost per accepted
  change, regression trends.

## 9. Success metrics

Do not optimise for “successful runs”; optimise for accepted, proven changes.

| Metric | Baseline (Oct 2026) | Phase A target | Phase D target |
|---|---|---|---|
| Time to first governed PR | hours + manual scopes | < 10 min | < 5 min |
| AC criteria with valid proof | 0% (`acceptance` = file presence) | > 80% | > 95% |
| Proof validity (automated + valid manual) | not measured | > 90% | > 98% |
| Escaped defect rate | not measured | measured | < 10% |
| Rework rate | not measured | measured | declining |
| Human intervention rate | high (observed) | measured | < 20% |
| Retry cost | not measured | reported per run | budgeted |
| Policy violation rate | measured per design check | reported | < 2% |
| First-pass PR success | unmeasured (T-21 missed) | > 60% on a defined cohort | > 80% |
| Merge lead time (p50) | hours | < 2 h | < 45 min |
| Concurrent runs per org | 1 | 4 | 20+ |
| Cost per accepted change | ledger only | reported | budgeted per project |

`First-pass PR success` only counts tasks with structured ACs and excludes
incidents/retries; the cohort and definition must be written before the metric is
used for decisions.

## 10. Risks and mitigations

| Risk | Mitigation |
|---|---|
| “Evidence theatre” — tests pass without proving the AC | base-SHA negative guard, test must be in diff, manual fallback audited |
| Visual checks produce false positives | masking, thresholds, separate signal, reviewer baseline acceptance |
| Context manifest becomes bureaucracy | auto-derive, show only deltas/conflicts, one-click confirm |
| Attachments become a security hole | scan, size/type limits, ACL, retention, signed URLs |
| Agent capability lags verification | repo index + MCP + alternative backend (A6) |
| Parallelism creates conflicts | worker isolation, idempotency, merge queue before decomposition |
| Audit is not tamper-evident | hash chain + signed export, not just append-only policy |
| Model/provider changes break agents | backend abstraction + contract tests |
| Sandbox escape or egress abuse | network policy, read-only mounts, resource caps, egress allowlist |
| Scope creep into an IDE | explicit non-goals; integrate with editors instead |
| Enterprise features slow the core loop | phase gates; no D work before C exit criteria |

## 11. Non-goals

- building a model or an IDE
- replacing GitHub/GitLab as the source of truth
- a no-code/low-code application builder
- autonomous merges without a human approval (stays true at every phase)
- mutation testing as a P0 gate (optional for critical modules later)

## 12. Sequencing and approval

Order:

1. A2 agent reliability (starts immediately, small)
2. A1 proof contract + negative guard (critical)
3. A3 context manifest v2 (attachments are a separate, security-reviewed story)
4. A4 e2e + visual evidence
5. A5 onboarding + preflight
6. A6 repo index/MCP/backend adapter (incremental)
7. B security/operations baseline
8. C worker pool → isolation → merge queue → dependency graph → monorepo →
   preview → provider parity
9. D enterprise (project RBAC and audit integrity first)
10. E product depth

Definition of “professional” used here: deterministic gates, provable
acceptance, versioned context, tenant isolation, horizontal scale, operational
runbooks and an extension surface — not the size of the feature list.

Implementation starts only after this revision is approved. The first PR covers
**A2 + A1** only; each following item gets its own story, acceptance criteria and
rollback note in `docs/05-mvp-plan.md` or a follow-up document. Before each item,
check what already exists in code — several capabilities are partial, not absent.
