# 00 · Product

## 1. Problem

Teams today have AI assistants, but no **system**. The result: code is born in chat
windows, context is lost, nobody knows which rules the agent used, tests are
skipped, and review becomes a bottleneck. No audit, no reproducibility, no
cost control.

## 2. Solution

A platform that treats AI agents as **executors within a defined process**, not as
chat partners:

- **Blueprint** defines the technologies, design system, test strategy, and rules for a specific
  class of applications. A new project is not "designed from scratch" — it is instantiated from the blueprint.
- **Backlog** is the only entry point for work. A task carries acceptance criteria and a definition of ready.
- **Agent run** is a deterministic state machine with phases, a budget, a sandbox, and
  checkpoints. Every step is recorded and reproducible.
- **Human gates** exist only where a decision carries risk: plan, merge, production deploy.
- **Context & Rules** are versioned; the agent run in the audit shows the exact versions.

> What this looks like in practice, at one company and for one task:
> [usecase.md](usecase.md) — day 0, from `T-142` to production in 6 hours, with the exact roles.

## 3. Users

| Persona | Goal | What they get |
|---|---|---|
| **Tech lead (Marko)** | to have the team ship faster without losing control | overview of 5–10 runs, plan/PR approval, diff and tests in one place |
| **Senior dev (Ana)** | not to spend a day on boilerplate | the agent does the scaffolding and the first test pass, she does the review |
| **Founder / PM (Jovan)** | to see progress and cost | dashboard KPIs, analytics, usage per project, without touching code |
| **Admin (IT)** | security and compliance | RBAC, audit log, provider keys, sandbox policy, SSO |

## 4. Modules (and where they appear in the UI)

| Module | Description | Screens |
|---|---|---|
| Organization & Workspace | multi-tenant, members, roles, audit | `/settings/*`, topbar switcher |
| Project Management | projects, backlog, tasks | `/projects`, `/projects/:id`, `/projects/:id/backlog` |
| Blueprint & Templates | definition of an application class + wizard | `/templates` |
| Design System & Components | tokens, components, versions | `/design-system`, `/components` |
| AI Agent Orchestration | runs, phases, live log, diff, approval | `/agents` (org-wide list), run pages under `/projects/:id/runs/:runId` |
| AI Context & Rules | rules per level (org → project → module → task) | `/settings/rules`, tab Context |
| Git Integration | repo connection, PRs, checks | `/repositories`, PR links |
| Automated Quality | test runs, coverage, gates | task detail, `/projects/:id` tab Tests |
| Environments & Delivery | dev/staging/prod, approval, release history | `/environments` |
| Analytics & Usage | KPIs, trends, cost | `/analytics`, dashboard |

Visual reference for all modules: [`design/mockups/`](../design/mockups/) — 18 screens
(HTML + PNG). The [design system](06-design-system.md) is authoritative; the mockups are illustrations.

## 5. Differentiators (what must not be lost in the MVP)

1. **Reproducible run** — from `run_id` every prompt, tool call, rules version, and
   diff can be reconstructed. Without this the product is not "engineering", but chat.
2. **`source: measured | estimated | n/a`** on every metric. False precision in
   analytics is disqualifying.
3. **Sandbox per run** — the agent physically cannot touch production or anyone else's repos.
4. **Budget** — the token/cost/time limit is part of the run contract, not "tracking afterwards".

## 6. MVP scope (and what we deliberately do not do)

**In the MVP:**
- one organization per user (multi-org exists in the model, but the UI is single-org)
- email+password and GitHub OIDC login
- GitHub **and GitLab** as git providers from the MVP (one `git.Provider` port, two adapters)
- one sandbox type (Docker on dev/self-host, K8s job in production)
- providers: any OpenAI-compatible endpoint + Anthropic (two adapters)
- blueprint wizard (5 steps), no marketplace
- plan approval, PR approval, prod deploy approval

**Outside the MVP (intentional, not accidental — plan and conditions in [roadmap-later.md](roadmap-later.md)):**
- real-time collaboration on documents
- mobile app
- third-party plugin SDK
- on-prem billing integrations
- automatic rollback based on SLOs (the MVP has manual rollback)

## 7. Glossary (used consistently in code and documentation)

| Term | Meaning in the system |
|---|---|
| **Blueprint** | Versioned description of an application class: stack, design system, test strategy, rules. Instantiates a project. |
| **Project** | An instance of a blueprint with its own repo, backlog, and environments. |
| **Task** | Unit of work in the backlog; carries acceptance criteria and (optionally) an agent run. |
| **Run** (`agent_run`) | A single agent execution over a task; a state machine with phases and a budget. |
| **Phase** | Step within a run (`context`, `plan`, `implement`, `test`, `review`, `diff`, `pr`). |
| **Rule** | Versioned text/policy that enters the agent's context (org/project/module/task level). |
| **Approval** | Human decision that unblocks the next phase (plan / PR / deploy). |
| **Artifact** | Run output: diff, log, test report, screenshot, plan document. |
| **Gate** | Condition that must be met for the process to move to the next phase. |
| **DoR / DoD** | Definition of Ready / Done — see [01-asdlc.md](01-asdlc.md#4-definition-of-ready-dor). |

## 8. Non-negotiable principles (they affect every decision)

1. **Determinism before intelligence.** If a decision can be written as a rule, it is
   written as a rule; the LLM is used only where a rule cannot.
2. **Humans decide, agents execute.** The agent never merges and never deploys to
   production without explicit approval.
3. **What is not recorded did not happen.** No tool call without a trace in
   `run_events`.
4. **Consistency before features.** Better 5 screens that work in all states than 15 that
   work only on the "happy path". See [06-design-system.md](06-design-system.md#4-state-machine).
