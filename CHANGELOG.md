# Changelog

All notable changes to this project. Dates are 2026.

## Unreleased

### Added

- **AI assistant (parts 1-3)**: the conversation store (migration 0083), the
  assistant service (the read/write tools behind a preview + confirmation, the
  untrusted tool-output wrapping, the per-user rate limit) and the endpoint —
  `POST /orgs/{org}/assistant/messages` with the JSON answer or the SSE
  `text`/`pending`/`done` events, plus the Anthropic adapter. The web panel
  follows.

- **Command palette**: `⌘K` (or `Ctrl+K`) opens a keyboard-first palette with
  the actions (projects, agents, search, settings, help), the projects, the
  searched tasks/runs and the recent runs; ↑/↓ navigate, Enter runs, Esc closes.
  The top-bar search opens it too.

- **Guidance layer**: a `NextAction` card on the run page (approve the plan,
  retry a failed development run, fix the review findings, review and merge),
  a `HelpTip` tooltip with a glossary, a help panel (the docs, the shortcuts and
  the glossary) in the top bar, the term tips in the task composer and the run
  summary, and an actionable empty state for the agent runs.

### Fixed

- The auth forms (login, register, invite accept, search), the create project
  form, the delivery panels (repo, providers, repository credential,
  environments) and the access panels (invites, API keys, rules) use the zod
  validation: inline field errors, the `aria-invalid` danger border and the
  server error at the form level; so do the agent plugin, design standard,
  preview, incident, SCIM token, GitHub App installation, backup request, SSO,
  workspace and BYOK forms, plus the project settings (the checks, the visual
  baseline, the delete confirmation, the policy fields, the MCP config, the
  plugin checks and the scaffold) (parts 1-4 of the forms consistency sweep
  #49).

### Added

- **Forms foundation**: react-hook-form + zod with a shared `useZodForm`, the
  field help/error, an unsaved-changes guard and a component-test setup
  (testing-library, jsdom). The task composer and the test/e2e/visual command
  settings are migrated: inline validation before submit and the form-level
  API errors.

- **Task scope fields and blueprint starter-task scope**: tasks carry the
  files, the out-of-scope, the proving test and the rollback (the
  Definition-of-Ready scope). The task composer and the drawer render them; the
  blueprint starter tasks accept the same scope and the instantiation creates
  them as draft tasks.

- **Task composer 2.0**: a full window for creating and editing a task — the
  acceptance criteria builder (multiple given/when/then rows), dependency
  picker, attachments, the live Definition of Ready, a card preview and three
  actions (Save draft, Create & ready, Create & start planning). The task
  update API accepts the acceptance criteria and dependencies.

## 0.1.1 — 2026-11-18

### Security

- **Dev dependency audit is clean**: vitest 3.2.7 → 4.1.12, vite 6.4.6, and a
  `@typescript-eslint/typescript-estree` 8.73 override (minimatch ^11.0.1).
  `pnpm audit` reports zero advisories. Dev-only: the shipped Next build is
  unchanged.

### Added

- **CI-fix runs**: a failed check on the pull-request page gains **Fix CI**;
  `POST /orgs/{org}/pull-requests/{pr}/checks/{check}/fix` starts a development
  run on the same plan and branch with the check log in the prompt (fetched on
  the host with the repository token). The review and the checks re-run.

- **Review-fix runs**: `POST /orgs/{org}/runs/{run}/fix-review` starts a
  development run that fixes the blocker findings of a finished review on the
  **existing pull request** (the same approved plan and branch; the review
  re-runs after the fix). The run page's review panel gains a "Fix review
  findings" action.

- **`aipctl seed`**: creates the demo org **Acme**, the owner `demo@acme.io`
  (password from `AIP_SEED_PASSWORD`), the **Demo** project and three ready
  tasks with acceptance criteria. Idempotent; `make seed` is documented in the
  quickstart.

- **Web dependencies in the offline sandbox**: when a repository has a
  `pnpm-lock.yaml` (root or `apps/web/`), the worker installs the locked
  dependencies on the host (`pnpm install --frozen-lockfile --ignore-scripts`)
  and the sandbox receives `node_modules` with the workspace. A
  `dependency.fetch` run event records it; `AIP_DEPS_NPM` (default on) and
  `AIP_PNPM_BIN` control the preparation.

- **Per-project test command**: `projects.test_command` overrides the global
  `AIP_TEST_COMMAND` (a Go API + Node web repository needs both toolchains in
  one gate). Delivery → Checks gains a test command field, the API exposes
  `PUT /orgs/{org}/projects/{project}/test-command`, and the worker prefers the
  project command, falling back to the global one.

- **Retry a failed development run with its approved plan**:
  `POST /api/v1/orgs/{org}/runs/{run}/retry` queues a new development run with
  the failed run's plan, moves the task back to `in_progress` and records a
  `run.retried` event — no planning run and no new approval. The run failure
  panel gains a "Retry development" action.

## 0.1.0 — 2026-11-17

### Added

- **Per-repository git credentials**: an encrypted PAT per repository that takes
  precedence over the GitHub App installation token and the global
  `AIP_GITHUB_TOKEN`; owner/admin-only API and Repositories UI, audit events,
  and a git-providers user guide (`docs/user/15-git-providers.md`).
- **CI check links**: check results keep the CI job URL (migration 0078); the PR
  checks panel links each check to its job page, so a failed check can be opened
  even when the check output is empty.
- **Offline dependency fetch**: development agents can call `fetch_module` to have
  the worker host download a Go module (through `AIP_DEPS_GOPROXY`, checksum
  verified) into the sandbox's module cache, so tasks that need a module no longer
  require a manual vendor PR. The sandbox stays offline.
- **The approved plan reaches development**: the development run now receives the
  planning run's plan (`run.Plan`), so the implementation agent works from the
  approved steps instead of only the task text.
- **Run failure UX**: the run API returns `failure_code`/`failure_detail`, the run
  page shows them with a **Retry task** button, and a failed run parks the task in
  `blocked` (instead of leaving it in progress) with the failure reason.
- **Configurable agent step budget**: `AIP_AGENT_MAX_STEPS` (default 24, range
  1-200) replaces the hard-coded 12-step tool-loop cap, and the development
  prompt now tells the agent to execute the whole approved plan before emitting
  `AIP_PROOF`.
- **ASDLC flow**: task → plan approval → development → test/design gates → PR →
  CI/human approval → merge → metrics, with SSE run events.
- **Enterprise controls**: project RBAC, append-only audit chain + witness, legal
  hold, SIEM delivery, policy-as-code, approval rules, change windows, merge
  queue, previews, incidents.
- **Identity**: OIDC + SAML SP (signed AuthnRequests, encrypted assertions, SLO),
  SCIM users/groups, per-org SSO settings, API keys.
- **BYOK**: `org_crypto`, per-organisation KMS, v4 envelope, encrypted KMS token,
  Settings UI, revoke drill.
- **Custom agents**: OCI-image registry, digest allowlist, project selection,
  worker execution, Settings UI, cosign verifier, fake-GitLab e2e.
- **HA/ops**: Patroni platform database (etcd + 2 Spilo + HAProxy), failover,
  partition/quorum/replica drills, logical backup, cold volume backup and
  standalone boot restore, gated sync-key recovery.
- **PITR**: durable MinIO WAL archive + base backups (wal-g, 30/30 retention,
  credentials in a 0600 env file) + restore from the base to a timestamp between
  two marker writes (docs/60/63).
- WAL archive metrics and Prometheus alerts for failures/staleness.
- PITR restore request API/UI and a worker-driven restore service; DR dashboard
  with restore success/last-age/duration SLOs and a 30-day stale-restore alert.
- Offsite MinIO mirror with heartbeat and 48h stale alert; restore drill from
  the offsite endpoint succeeded.
- Real Git rebase conflict test for the merge queue (conflict blocks merge).
- GitLab merge-request rebase support with async conflict detection.
- K8s sandbox: read-only root, seccomp, bounded emptyDir volumes and MCP exec.
- K8s live drill on kind: read-only root, writable workspace and MCP stdio passed.
- Unsupported egress allowlist env now fails startup instead of being ignored.
- WAL scripts moved into the Patroni data volume after a recreate stopped archiving.
- Base-backup script/heartbeat, 48h staleness alert and a cron install artifact.
- Vault-dev root-token and AppRole secret sync/rotation PoCs for WAL
  credentials (docs/61/62).
- **Multi-region**: opt-in `AIP_REGION` routing guard (421 wrong_region).
- **UI**: visual baseline-vs-current, structured retry panel, blueprint templates,
  per-project onboarding timer.
- **Success metrics**: first-pass merge, proof validity, dev runs/task and an
  escaped-defect proxy in `/metrics` and Analytics.
- **Concurrency**: ten-task API e2e with ten distinct merged PRs (docs/57).
- **Supply chain**: cosign in the API image, JS license allowlist, govulncheck.

### Changed

- Platform API/worker run on Patroni through `patroni-haproxy`; the old Postgres
  remains a pre-cutover rollback snapshot.
- API/worker use HAProxy → PgBouncer (prepared-statement tracking).
- Per-org worker fairness uses Postgres advisory-lock slots
  (`AIP_ORG_MAX_CONCURRENT_RUNS`).

### Security

- The web dev/test toolchain was patched: vitest 2.1.9 → 3.2.7 (the UI-server
  advisory) and a tinypool `^2.1.2` override (the prototype-pollution RCE).
  `pnpm audit --prod` is clean; the remaining advisories are dev-only (vitest
  mocks, picomatch) and tracked.

### Fixed

- The run page refreshes on `awaiting_approval`, so a finished planning run
  shows its plan and approval panel without a manual reload (the SSE stream does
  not close for a non-terminal state).
- The tool loop reserves the final model step for a final answer (tools are
  not offered on that step), so the agent emits the required `AIP_*` line
  instead of spending every step on tools and failing `acceptance_unproven`.
- The review sees diffs up to 200 KB (was 60 KB) and gets a changed-file index;
  a truncated diff carries a caveat. The T-4 diff (126 KB) had its handler hunks
  cut off, which produced two false major findings.
- The secret scanner ignores deliberate test fixtures (`never-issued`, `dummy`,
  `fake`, `fixture`, …) and lines carrying the `aip:allow-secret` pragma, so a
  fake token in a test no longer fails the run; real-looking tokens still fail.
- A missing dependency in the offline sandbox is classified as
  `dependency_missing` (instead of `test_gate_failed`); the development prompt
  tells the agent to stop and report instead of reimplementing a library, and
  the review flags hand-written cryptography and agent-authored vendor code.
- `RepoMap` no longer panics on files without an extension (`LICENSE`,
  `Makefile`): they are skipped instead of sliced at index -1.
- The run event stream (SSE) flushes through the metrics middleware again; the
  UI was missing `run.awaiting_approval` and could not show plan approval.
- The review agent receives the development diff and reviews the change, not the
  cloned base repository; `acceptance_proof` and `review` artifacts are allowed
  by the `run_artifacts` kind constraint.
- Git push and clone errors redact the token embedded in the remote URL.
- A failed branch push records a `pr.failed` event with the provider error
  instead of only a log warning.
- GitHub rebase no longer sends an empty `expected_head_sha` (GitHub answered
  422); an already up-to-date branch is treated as success.
- Deterministic frozen context: tar metadata/order and agent type no longer change
  the hash (e2e no longer fails `context_changed`).
- `GO-2026-4753`: upgraded goxmldsig to 1.6.0 (SAML signature bypass).
- Typed-nil KMS interface ("kms: not configured").
- e2e no longer runs `docker compose down -v` against the developer stack.
- `.git/` no longer changes the frozen context hash under concurrent pushes.
- `visual/*` artifacts are allowed by the DB check and stored as PNGs

### Verified

- `make check`, `make security`, `make e2e` green.
- Patroni: Leader running, Replica streaming lag 0 MB; API connects through
  Patroni (6 connections observed).
- Data snapshot: 12 orgs / 12 projects / 330 tasks / 374 runs.
- Sandbox: no caps, no mounts, no network, read-only root, writable tmpfs.

### Known gaps

- Host fencing: no `/dev/watchdog`, systemd offline, no cloud STONITH
  (docs/56).
- Sync replication: stale `/service/<scope>/sync`; sync mode stays off, manual
  gated recovery only (docs/53).
- Multi-region: routing guard only; no second data plane.
- Volume restore boots standalone, not as a Patroni node (docs/51).
- Backup/PITR, live embedding provider, seccomp/AppArmor and K8s sandbox are not
  drilled.

See [docs/54-status.md](docs/54-status.md) for the authoritative status and
[docs/28-enterprise-roadmap.md](docs/28-enterprise-roadmap.md) for the plan.

## Open-source preparation

- Apache-2.0 license, NOTICE, security/contribution/governance policies,
  issue/PR templates and `make oss-audit`.
- Public-readiness plan in `docs/OPEN_SOURCE.md`; the repository is not published
  yet and the history still needs a secret scan.
