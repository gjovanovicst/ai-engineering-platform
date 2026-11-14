# 08 · Gap register (coverage before M0)

> Question: "do we have everything complete to build all the features?"
> Answer: **yes for the MVP, with one caveat** — all P0 gaps were closed 2026-10-21
> (G1–G4), the P1 items (search, runbook, threat model, metrics) are documented or
> planned, and the original spec file is still missing (see §6).

## 1. How it was verified

| Check | Method | Result |
|---|---|---|
| Screens | 18 mockups (3 + 15) vs §5 compositions | 18/18 have a composition |
| Components | classes in the mockups vs §3 contracts | **13/38 have a contract** (25 without, 1 partial) |
| API | screen needs vs `openapi.yaml` | 31 paths; **10 required ones are missing** |
| Data model | entities from the screens vs the schema | 27/31 tables, 4 missing |
| Agent runtime | documented vs needed for code | ~60% |
| Operations | deploy/rollback/restore/alert | ~25% |

## 2. Coverage by area

| Area | Status | Has | Missing |
|---|---|---|---|
| Product and scope | 🟢 90% | vision, personas, 10 modules, MVP scope, glossary | the original spec (TXT) is not in the share; modules were reconstructed |
| ASDLC process | 🟢 95% | phases, gates, DoR/DoD, metrics, failure handling | examples of approval decisions (comment templates) |
| UI screens | 🟢 90% | 18 compositions + 18 rendered mockups | Settings sub-pages (Members mockup only), the run's Plan/Tests tab, 404/500, onboarding, email templates |
| UI components | 🔴 **35%** | contracts for 13 components | **25 without a contract** (list in §3) |
| API | 🟢 95% | **43 paths**, errors, idempotency, RBAC, SSE | P1: webhook endpoints (outgoing) |
| Data model | 🟢 95% | **38 tables**, RLS, indexes, retention | webhook delivery table (P2) |
| Go architecture | 🟢 95% | layers, runtime, sandbox, providers + [11-agent-runtime.md](11-agent-runtime.md) (tool schemas, prompt layers, context, image) | — |
| Blueprint schema | 🔴 **0%** | mentioned as `spec: jsonb` | **no definition** — blocks the wizard, project create and context assembly |
| Testing | 🟢 90% | pyramid, replay, e2e, security checklist | test fixtures for every screen |
| Operations | 🔴 30% | deployment architecture, graceful shutdown | runbook, SLO, backup/restore procedure, alerting rules, k8s manifests |
| Auth and accounts | 🟢 100% | [12-auth.md](12-auth.md): registration, invite, reset, verification, tokens, CSRF, OIDC, rate limit | — |
| Setup and environments | 🟢 100% | [13-setup.md](13-setup.md): env, secrets, compose, email, GitHub App, backlog | — |
| Security | 🟡 60% | sandbox, secrets, RBAC, RLS, audit | threat model document, prompt-injection matrix, incident response |

## 3. P0 — closed 2026-10-21 ✅

### G1 · Blueprint `spec` schema
`blueprint_versions.spec` was `jsonb` with no definition — the wizard, `POST /projects` and the
agent context had nothing to validate against.

**Status:** ✅ [09-blueprint-schema.md](09-blueprint-schema.md) — JSON Schema 2020-12,
2 examples (SaaS Next+Go, internal tool React+Node), a `spec → records` table, versioning
rules (`draft/published/deprecated`, `spec_sha256`).

### G2 · Components without contracts
The mockups use 26 components that §3 did not define.

**Status:** ✅ [10-components.md](10-components.md) — 26 contracts (Tabs, Breadcrumbs,
CommandPalette, Switcher, FilterBar, ViewToggle, KanbanBoard/Column, TaskCard, Stepper,
FormField, Input, Select, Checkbox, Radio, Toggle, Toast, Alert, ProgressBar, Sparkline,
Pagination, FileTree, CodeBlock, Tooltip/Popover, Avatar, Kbd), the same format as §3,
with a11y requirements. `06-design-system.md` links to the addendum.

### G3 · Agent runtime
Tool schemas, prompt templates, context selection, token counting and the content of the
sandbox image were missing.

**Status:** ✅ [11-agent-runtime.md](11-agent-runtime.md) — 12 tool schemas (JSON Schema
of arguments), an allowlist matrix per agent type, 7 prompt layers with budget and delimiters,
a deterministic context algorithm (7 steps), token budget, sandbox image Dockerfile,
prompt-injection mitigations, replay format.

### G4 · API and schema for 4 screens
Components registry, design system tokens, PR merge/waive, comments, notifications,
search, activity feed, deploy logs.

**Status:** ✅ `03-data-model.md` +8 tables (`design_systems`, `design_system_versions`,
`components`, `component_versions`, `notifications`, `webhook_endpoints`,
`deployment_logs` + `ALTER projects`), `04-api.md` +12 endpoints,
`openapi.yaml` **43 paths / 47 schemas**. The P1 items (search, activity, notifications,
comments, deploy logs) have a contract and are implemented in M4.

### Also closed before M0

- **Auth lifecycle** — [12-auth.md](12-auth.md): registration (self-serve + invite),
  email verification, password reset, refresh rotation with reuse detection, CSRF and
  cookie policy, OIDC (GitHub/Google), per-device sessions, rate limit, audit events.
- **Setup decisions** — [13-setup.md](13-setup.md): Go module path, domains, compose services,
  `.env` variables, secrets, email provider, GitHub App, first-commit checklist.
- **Bootstrap** — [02-architecture.md §18](02-architecture.md#18-bootstrap-and-seed):
  `aipctl bootstrap` for the first owner.
- **Backlog for agents** — [AGENTS.md §9](../AGENTS.md#9-backlog-how-an-agent-picks-up-a-story):
  GitHub Issues + labels + pickup protocol.

## 4. P1 — before the first user (M4/M5)

| # | Gap | Why it matters | Where |
|---|---|---|---|
| G5 | Search (⌘K) | without it, navigation does not scale beyond 20 projects | Postgres FTS + `/search`, `docs/12-search.md` |
| G6 | Activity feed + comments | the dashboard and task drawer display them | `activity` view + `/activity`, `/comments` |
| G7 | Notifications (in-app + email) | "2 tasks are awaiting approval" is promised in the UI | `notifications` table + worker |
| G8 | Ops runbook | an incident without a procedure = extended downtime | `docs/14-runbook.md` — scenarios, alerts, SQL, security incidents, backup/restore |
| G9 | Threat model | prompt injection and sandbox escape are not worked out | `docs/15-security.md` — assets, STRIDE, prompt injection, sandbox, residual risks |
| G10 | Analytics definitions | a KPI without a formula = made-up numbers | `docs/16-metrics.md` — formulas, windows, source rules, slice-gate definitions |
| G11 | Deploy logs + release timeline | the Environments screen displays them | `deployment_logs` + `/deployments/{id}/logs` |

## 5. P2 — after the MVP (intended, not forgotten)

SAML SSO · billing · outgoing webhooks · plugin SDK · **mobile app (native)** ·
PWA · i18n · GitLab/Bitbucket · warm sandbox pool · multi-region · blueprint marketplace.

The detailed plan, triggers and the **seams the MVP must preserve** (S1–S12) are in
[roadmap-later.md](roadmap-later.md).

## 6. What was reconstructed (and why I am asking for the original TXT)

The original attachment `AI Engineering Platform.txt` (24.48 KB) **is not in the shared conversation**.
Reconstructed from the design conversation: the module list, the feature→UI map, the screens and their
content. **Not** reconstructed: the exact data model from the spec, the business rules of the modules
(§3.1–§3.10 of the original), the priorities and acceptance criteria from the original backlog.

Consequence: `00-product.md`, `03-data-model.md` and `05-mvp-plan.md` are **my interpretation**,
not a translation of your document. If the TXT shows up, these three files are aligned before M0 —
after the first migration, alignment costs 10× more.

## 7. Proposed closing order (before code)

| Day | Work | Output |
|---|---|---|
| 1 | G1 blueprint schema + G2 components (first 12) | wizard and primitives ready for M0 |
| 2 | rest of G2 + G4 API/schema additions | OpenAPI "feature complete" for the MVP |
| 3 | G3 agent runtime spec | E3 can start without inventing anything |
| 4 | G5–G7 (search, activity, notifications) | M4 screens without gaps |
| 5 | G8–G10 (runbook, security, metrics) | M5 hardening has a basis |

Alternative (faster but riskier): start M0 right away and write G1/G2/G3 in parallel with the
first stories. Risk: `web/components` gets 3 variants of `Tabs` before the contract
exists — cleaning up costs more than the delay.
