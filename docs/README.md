# Documentation

Start with the [README](../README.md) for the product overview, or the [quickstart](quickstart.md) to run it locally. The [user guide](../site/docs/) covers setup, projects, tasks, runs, approvals and the API reference.

## Start here

| Document | What it covers |
|---|---|
| [Quickstart](quickstart.md) | Run the platform locally: compose, migrations, sandbox image, first project |
| [00-product](00-product.md) | Vision, modules, MVP scope and glossary |
|X|Product |
| [01-asdlc](01-asdlc.md) | The lifecycle the platform implements: phases, gates, artifacts, roles, metrics |
|X|ASDLC — AI Software Development Lifecycle |
| [05-mvp-plan](05-mvp-plan.md) | Epics, stories, acceptance criteria and milestones |
|X|MVP plan |
| [usecase](usecase.md) | Use case — one company, practical use |

## Architecture, data and API

| Document | What it covers |
|---|---|
| [api/openapi.yaml](../api/openapi.yaml) | The HTTP contract (OpenAPI 3.1) |
| [api/events.md](../api/events.md) | The SSE event catalog of a live run |
| [02-architecture](02-architecture.md) | Go packages, agent runtime, sandbox, providers, security, ops |
|X|Architecture (Go) |
| [03-data-model](03-data-model.md) | ER model and the complete Postgres schema |
|X|Data model |
| [04-api](04-api.md) | REST conventions, errors, pagination, idempotency, RBAC |
|X|API |
| [09-blueprint-schema](09-blueprint-schema.md) | Blueprint JSON Schema and project instantiation |
|X|Blueprint schema |
| [10-components](10-components.md) | Component contracts (supplement to the design system) |
|X|Component contracts (supplement to §3) |
| [11-agent-runtime](11-agent-runtime.md) | Tool schemas, prompt layers, context assembly, sandbox image |
|X|Agent runtime spec |
| [12-auth](12-auth.md) | Accounts, registration, invite, reset, tokens and CSRF |
|X|Auth & account lifecycle |
| [13-setup](13-setup.md) | Environments, env, secrets, backlog and the first-commit checklist |
|X|Project setup — decisions before the first commit |

## Quality, security and the agents

| Document | What it covers |
|---|---|
| [07-testing](07-testing.md) | Test strategy, fixtures and the replay harness |
|X|Testing |
| [15-security](15-security.md) | Threat model: trust boundaries, STRIDE, prompt injection, sandbox |
|X|Threat model |
| [31-threat-model](31-threat-model.md) | Phase-B threat model and residual risks |
|X|Threat model (Phase B) |
| [32-audit-integrity](32-audit-integrity.md) | Append-only audit design and verification |
|X|Audit integrity design |
| [55-sandbox-hardening](55-sandbox-hardening.md) | Agent sandbox hardening drill |
|X|Agent sandbox hardening drill |
| [73-egress-policy](73-egress-policy.md) | Sandbox egress policy (fail-closed) |
|X|Sandbox egress policy |
| [19-a11y](19-a11y.md) | Accessibility audit: findings, fixes, known gaps, manual test script |
|X|Accessibility audit |
| [26-design-standards-plan](26-design-standards-plan.md) | Design standards and the component registry (proposal) |
|X|Design Standards & Component Registry (proposal) |
| [39-adr-saml](39-adr-saml.md) | ADR 0001 — SAML login: a library, not hand-rolled |
|X|ADR 0001 — SAML login: library, not hand-rolled |
| [44-adr-custom-agents](44-adr-custom-agents.md) | ADR 0002 — custom agent plugins |
|X|ADR 0002 — Custom agent plugins |

## Operations

| Document | What it covers |
|---|---|
| [14-runbook](14-runbook.md) | On-call runbook: alerts, failure scenarios, SQL, security, restore |
|X|Runbook |
| [16-metrics](16-metrics.md) | Metric definitions, windows and the slice-gate formulas |
|X|Metric definitions |
| [17-pilot-checklist](17-pilot-checklist.md) | Pilot go/no-go: repository, environment, security, data, product, rollback |
|X|Pilot checklist |
| [18-load-testing](18-load-testing.md) | Load and soak harness, baseline numbers and scaling rules |
|X|Load and soak testing |
| [29-backup-restore](29-backup-restore.md) | Backup, restore and the drills behind them |
|X|Backup and restore |
| [33-operations-runbook](33-operations-runbook.md) | Production operations runbook |
|X|Operations runbook |
| [38-ha-production](38-ha-production.md) | Production Postgres HA design |
|X|Production Postgres HA — design |
| [47-multi-region](47-multi-region.md) | Multi-region design |
|X|Multi-region design |
| [52-patroni-fencing](52-patroni-fencing.md) | Patroni host fencing |
|X|Patroni host fencing |
| [60-durable-wal-pitr](60-durable-wal-pitr.md) | Durable WAL archive and PITR from MinIO |
|X|Durable WAL archive + PITR from MinIO |
| [71-kubernetes-sandbox](71-kubernetes-sandbox.md) | Kubernetes sandbox hardening and MCP process |
|X|Kubernetes sandbox hardening and MCP process |
| [72-kubernetes-live-drill](72-kubernetes-live-drill.md) | Kubernetes sandbox live drill |
|X|Kubernetes sandbox live drill |

## Integrations and real-world setup

| Document | What it covers |
|---|---|
| [21-real-github-test](21-real-github-test.md) | End-to-end test with a real GitHub repository, from zero to a merged PR |
|X|End-to-end test with a real GitHub repository |
| [22-deepseek-github](22-deepseek-github.md) | Real setup with DeepSeek + GitHub, including pricing and troubleshooting |
|X|Real use: DeepSeek + GitHub |
| [27-github-app](27-github-app.md) | GitHub App setup |
|X|GitHub App setup |
| [30-mcp-and-agent-adapter](30-mcp-and-agent-adapter.md) | MCP tools and external agent adapters |
|X|MCP tools and external agent adapters |
| [36-keycloak-sso](36-keycloak-sso.md) | Keycloak SSO for development |
|X|Keycloak SSO (dev) |
| [57-concurrent-pr-e2e](57-concurrent-pr-e2e.md) | Ten concurrent PR end-to-end |
|X|Ten concurrent PR e2e |
| [69-real-git-conflict](69-real-git-conflict.md) | Real Git rebase conflict in the merge queue |
|X|Real Git rebase conflict in the merge queue |
| [70-gitlab-rebase](70-gitlab-rebase.md) | GitLab merge-request rebase |
|X|GitLab merge-request rebase |

## Status and roadmap

| Document | What it covers |
|---|---|
| [54-status](54-status.md) | Verified status, topology and blockers |
|X|Project status — 2026-11-05 |
| [28-enterprise-roadmap](28-enterprise-roadmap.md) | Enterprise readiness roadmap |
|X|Enterprise readiness roadmap |
| [roadmap-later](roadmap-later.md) | P2 plan (mobile, PWA, GitLab, billing) and the seams the MVP preserves |
|X|Roadmap — "later" (P2) |
| [OPEN_SOURCE](OPEN_SOURCE.md) | Open-source readiness |
|X|Open-source readiness |
| [ui-redesign-handoff](ui-redesign-handoff.md) | UI redesign handoff |
|X|UI Redesign Handoff |

## Repository guides

| Document | What it covers |
|---|---|
| [AGENTS.md](../AGENTS.md) | Rules for the AI agents building this repository |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | How to contribute, DCO |
| [SECURITY.md](../SECURITY.md) | How to report a vulnerability |
| [CHANGELOG.md](../CHANGELOG.md) | Releases and known gaps |
| [design/mockups/](../design/mockups/) | 18 screens as HTML + PNG (visual reference) |
| [site/](../site/) | GitHub Pages site: overview, workflow, quickstart, architecture, status |
