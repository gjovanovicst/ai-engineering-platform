# AI Engineering Platform

A platform where AI agents execute the complete software lifecycle: from idea and
blueprint, through backlog, planning, implementation and testing, to review,
merge and deploy, with a human at the checkpoints and a full audit trail.

> **New here?** Start with the [quickstart](docs/quickstart.md) for a local
> docker-compose setup, then read the documentation below.

> **The backend is Go.** The frontend is Next.js (App Router). The database is PostgreSQL. This is not
> a preference that can change mid-flight. See [ADR-0001](docs/adr/README.md).

## Documentation (read in this order)

| # | Document | What it provides | For whom |
|---|---|---|---|
| 1 | [docs/00-product.md](docs/00-product.md) | Vision, modules, MVP scope, glossary | everyone |
| 2 | [docs/01-asdlc.md](docs/01-asdlc.md) | **ASDLC**, the lifecycle the platform implements: phases, gates, artifacts, roles, metrics | product + backend |
| 3 | [docs/02-architecture.md](docs/02-architecture.md) | Go architecture: packages, runtime, sandbox, providers, security, ops | backend |
| 4 | [docs/03-data-model.md](docs/03-data-model.md) | ER model + complete Postgres schema | backend |
| 5 | [docs/04-api.md](docs/04-api.md) | REST conventions, errors, pagination, idempotency, RBAC | backend + frontend |
| 6 | [api/openapi.yaml](api/openapi.yaml) | Machine-readable contract (OpenAPI 3.1) | everyone |
| 7 | [api/events.md](api/events.md) | SSE event catalog (live run) | backend + frontend |
| 8 | [docs/05-mvp-plan.md](docs/05-mvp-plan.md) | Epics, stories, acceptance criteria, milestones | everyone |
| 9 | [docs/06-design-system.md](docs/06-design-system.md) | UI spec: tokens, 13 components, screens, a11y | frontend |
| 10 | [docs/09-blueprint-schema.md](docs/09-blueprint-schema.md) | Blueprint JSON Schema + project instantiation | backend + product |
| 11 | [docs/10-components.md](docs/10-components.md) | 26 component contracts (supplement to §3) | frontend |
| 12 | [docs/14-runbook.md](docs/14-runbook.md) | On-call runbook: alerts, failure scenarios, SQL, security, restore | everyone |
| 13 | [docs/15-security.md](docs/15-security.md) | Threat model: trust boundaries, STRIDE, prompt injection, sandbox, residual risks | backend + security |
| 14 | [docs/16-metrics.md](docs/16-metrics.md) | Metric definitions, windows, source rules and the slice-gate formulas | everyone |
| 15 | [docs/17-pilot-checklist.md](docs/17-pilot-checklist.md) | Pilot go/no-go: repository, environment, security, data, product, rollback | everyone |
| 16 | [docs/18-load-testing.md](docs/18-load-testing.md) | Load/soak harness, baseline numbers, scaling rules | backend |
| 17 | [docs/19-a11y.md](docs/19-a11y.md) | Accessibility audit: findings, fixes, known gaps, manual test script | frontend |
| 18 | [CHANGELOG.md](CHANGELOG.md) | Release notes and known gaps | everyone |
| 19 | [docs/54-status.md](docs/54-status.md) | Verified status, topology and blockers | everyone |
| 20 | [site/](site/) | GitHub Pages site: overview, workflow, quickstart, architecture, status | everyone |
| 20 | [docs/21-real-github-test.md](docs/21-real-github-test.md) | End-to-end test with a real GitHub repository, from zero to a merged PR | everyone |
| 21 | [docs/22-deepseek-github.md](docs/22-deepseek-github.md) | Real setup with DeepSeek as the provider + GitHub, incl. pricing and troubleshooting | everyone |
| 12 | [docs/11-agent-runtime.md](docs/11-agent-runtime.md) | Tool schemas, prompt layers, context assembly, sandbox image | backend |
| 13 | [docs/12-auth.md](docs/12-auth.md) | Accounts, registration, invite, reset, tokens, CSRF | backend + frontend |
| 14 | [docs/13-setup.md](docs/13-setup.md) | Environments, env, secrets, backlog, checklist for the first commit | everyone |
| 15 | [docs/07-testing.md](docs/07-testing.md) | Test strategy, fixtures, replay harness | everyone |
| 17 | [design/mockups/](design/mockups/) | 18 screens as HTML + PNG (visual reference) | frontend |
| 18 | [docs/roadmap-later.md](docs/roadmap-later.md) | P2 plan (mobile, PWA, GitLab, billing…) + **seams the MVP preserves** | everyone |
| 19 | [docs/usecase.md](docs/usecase.md) | How a company uses the platform: day 0, a task from plan to prod, the weekly rhythm | everyone |
| 20 | [AGENTS.md](AGENTS.md) | Rules for the AI agents building this repository | AI |

## Stack (a decision, not a proposal)

```
Frontend   Next.js 15 (App Router) · React 19 · TypeScript strict · Tailwind 3.4 · TanStack Query
Backend    Go 1.25+ · chi/v5 · oapi-codegen · pgx/v5 · sqlc · River (job queue) · go-git
Data       PostgreSQL 17 · Redis 7 (SSE fanout, rate limit) · S3/MinIO (artifacts)
Agents     internal runtime in Go (state machine + tool calls), provider adapters
Infra      Docker · docker compose (dev) · Kubernetes (prod) · GitHub Actions
Obs        OpenTelemetry · Prometheus · slog (JSON)
```

## Repository structure

```
api/
  openapi.yaml              # single source of truth for the HTTP contract
  events.md                 # SSE event catalog

apps/
  api/                      # Go module (backend)
    cmd/aipd/               # HTTP API
    cmd/aip-worker/         # River worker + agent runtime
    cmd/aipctl/             # CLI (admin, migrations, replay)
    internal/
      domain/               # entities + invariants, no I/O
      app/                  # use cases (transactions, orchestration)
      agent/                # runtime, phases, tools
      adapters/             # postgres, s3, redis, llm, sandbox, git, github, oidc
      httpapi/              # router, middleware, handlers, DTO
      platform/             # log, config, db, httpsse, clock
    migrations/             # goose SQL migrations
  web/                      # Next.js app (E2-S1)

deploy/                     # postgres roles, docker, k8s (M5)
docs/                       # product, process, architecture
design/                     # mockups (HTML + PNG), not part of the build
```

## First steps (after M0-S1)

> Status: the vertical slice works end to end — task → plan → approval → development
> (diff + tests) → review → branch/PR → PR approval gate → merge → done, with GitHub and
> GitLab, analytics, invites, OIDC, tracing and a Kubernetes sandbox driver. See the
> status table in [docs/05-mvp-plan.md](docs/05-mvp-plan.md).

```bash
cp .env.example .env          # DSNs, keys; details in docs/13-setup.md §4
make tools                    # goose, sqlc, oapi-codegen
docker compose up -d          # postgres, redis, minio, mailhog
make keygen                   # Ed25519 key for JWT (jwt.pem)
make migrate-up               # schema + RLS
make dev                      # aipd :8080 + aip-worker
make web-install && make web-dev   # Next.js on :3000
make check                    # fmt + vet + lint + test
```

Verify that it works:

```bash
curl localhost:8080/healthz          # {"status":"ok"}
curl localhost:8080/api/v1/version   # {"version":"...","uptime":"..."}
```

## How this is built

The repository is developed with **the ASDLC from [docs/01-asdlc.md](docs/01-asdlc.md)**, the same
process the platform sells. Every story from [docs/05-mvp-plan.md](docs/05-mvp-plan.md)
goes through the same gates: DoR → plan → implementation → tests → review → merge.
Dogfooding is not a marketing phrase: if the process does not work on us, it does not work for users either.

## MVP definition of success

A single user can do the following without manual work in a terminal:

1. create an organization and a project from a blueprint,
2. enter a task in the backlog,
3. start the Planning agent and approve the plan,
4. have the Development agent write code in the sandbox, run tests and open a PR,
5. see the live log, diff, usage and exactly which rule version the agent used,
6. merge the PR and deploy to staging through the approval gate.

Everything else comes after the MVP.


## Open source

Licensed under [Apache-2.0](LICENSE). Contributions are accepted under the
Developer Certificate of Origin; see [CONTRIBUTING.md](CONTRIBUTING.md).

- security reports: [SECURITY.md](SECURITY.md)
- governance: [GOVERNANCE.md](GOVERNANCE.md)
- public-readiness plan: [docs/OPEN_SOURCE.md](docs/OPEN_SOURCE.md)
- user guide and API reference: [site/docs/](site/docs/)

The status is deliberately honest: the core ASDLC is verified, while fencing,
sync RPO and multi-region are not production-ready. See
[docs/54-status.md](docs/54-status.md).
