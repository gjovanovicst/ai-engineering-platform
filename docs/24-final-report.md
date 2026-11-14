# 24 · Delivery report (M0–M5)

Date: 2026-10-25 · Status: **50/50 stories done** (plus 3 items outside the plan)

## 1. What was built

One product, not a demo: a Go API + worker that execute the ASDLC loop, and a
Next.js web app.

| Area | Delivered |
|---|---|
| **ASDLC** | blueprints (versions), projects, tasks (refs T-N), board with 7 columns, plan approval, development run, CI checks, review blockers, merge gate, done |
| **Agents** | command backend, OpenAI-compatible chat, tool loop (read/list/grep/write/tests, 12 steps), Anthropic Messages API with the same loop; per-run token/cost/wall-clock budget |
| **Git** | host-side clone/push, GitHub + GitLab adapters, webhooks (HMAC/constant-time/dedupe), PR bodies from the task+plan, PR detail, merge, GitHub App installation tokens with PAT fallback |
| **Platform** | multi-tenant RLS, EdDSA JWT + rotating refresh, invites, roles (change/remove), providers with encrypted keys and prices, rules with diff + four-eyes publish, environments, deployments with four-eyes, audit, usage ledger, analytics, notifications (in-app + email), search |
| **Ops** | compose stack (postgres, redis, minio, mailhog, api, worker, migrate), goose migrations (28), River queue, S3 artifacts + retention archive, Redis SSE fan-out, per-org limits, k6 load test, security checklist |

## 2. Evidence

- `make check` green: fmt, vet, golangci-lint, unit + integration tests, tsc,
  eslint, OpenAPI diff.
- k6 load test: 50 runs / 154 requests, **p95 118.67 ms**, 0 failed requests.
- E2E: real GitHub repo, CI check run, merge gate, task done (PR #4/#5), plus
  provider-backed planning/development runs.
- Security: sandbox verified read-only/cap-drop/no-new-privileges; `govulncheck`
  0 code vulnerabilities (toolchain 1.27.3); `pnpm audit` clean.
- Migrations tested from empty database and applied on a populated demo DB.

## 3. How to run

```bash
cp .env.example .env      # fill secrets
docker compose up -d
docker compose run --rm migrate
./scripts/dev-web.sh
# optional webhook tunnel: see docs/21-real-github-test.md
```

Demo login is seeded by `scripts/seed-demo.sh`; use the value from that script or your own seeded account.

## 4. Known limits (honest)

- GitHub App path is unit-tested, not exercised against a live App.
- GitLab/OIDC adapters are verified against fakes/docs, not a live tenant.
- Web sessions are bearer tokens; no cookie+CSRF BFF yet.
- Rate limits and webhook replay dedupe are per replica.
- Retention archives `run_events` to S3 but does not delete them (append-only).
- No external penetration test.
- P2 scope (mobile/PWA, Bitbucket/Gitea, billing) was not started.

## 5. Next cycle candidates

1. Verify GitHub App + GitLab + OIDC against live tenants; document the runbook.
2. Cookie-based web auth with CSRF; move limits/dedupe to Redis/DB.
3. P2: mobile/PWA polish, Bitbucket/Gitea adapters, billing.
