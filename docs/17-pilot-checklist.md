# 17 · Pilot checklist

Use this before the first customer workspace. §1 is automated; everything else
needs a human with access to the environment.

## 1. Repository (automated)

```bash
make pilot-check
```

Checks: clean tree, no `.env`/`jwt.pem`/k8s secret committed, `make check`,
OpenAPI models in sync, every migration has a down path, no TODO/FIXME without
an issue, no raw colours in the design-system components.

## 2. Environment

- [ ] production domain + TLS on the ingress (`deploy/k8s/ingress.yaml`)
- [ ] Postgres reachable, `aip_app` has **NOBYPASSRLS**, `aip_migrator` used only by migrations
- [ ] `aipctl migrate up` reports the expected version; the migrate Job ran before the rollout
- [ ] `jwt.pem` generated for this environment (never reused from dev), mounted read-only
- [ ] SMTP credentials verified by sending a test invite
- [ ] `AIP_API_URL` and `AIP_WEB_URL` point at the public URLs (OIDC redirects depend on it)
- [ ] sandbox image pinned by digest and pullable by the cluster
- [ ] `AIP_OTEL_ENDPOINT` set and the collector reachable

## 3. Security

- [ ] `docs/15-security.md` reviewed with the customer's security owner
- [ ] git tokens scoped to the pilot repositories only
- [ ] webhook secret generated per repo and stored in the git host
- [ ] rate limits sane for the environment (`internal/httpapi` limits)
- [ ] audit log retention agreed
- [ ] incident contact and escalation path exchanged

## 4. Observability and data

- [ ] Prometheus scraping sql_exporter and the OTel collector (`deploy/observability/`)
- [ ] `AipRunQueueStuck`, `AipRunFailureRateHigh`, `AipWorkerDown`, `AipDbUnreachable` alerts routed to a human
- [ ] Grafana dashboard imported
- [ ] nightly backup CronJob succeeded at least once; `make restore-check` passed on the dump
- [ ] disk alerts on the backup PVC

## 5. Product readiness

- [ ] pilot users invited and roles checked (owner/admin/member/viewer)
- [ ] one repository connected end to end (webhook delivery green)
- [ ] one blueprint published with starter tasks
- [ ] one real task through plan → approval → development → review → PR → merge
- [ ] the PR gate is understood: merges require an in-platform approval
- [ ] users know where the run log and diff live
- [ ] support channel + response expectation agreed

## 6. Week-8 slice gate (data)

- [ ] `docs/16-metrics.md` §7 queries run against production with the raw output attached
- [ ] ≥ 10 users asked the "would miss it" question
- [ ] decision recorded: continue / narrow / pivot, with the numbers

## 7. Known gaps accepted for the pilot

List them explicitly in the pilot agreement; do not discover them in production:

- one git token per provider (no per-repo tokens)
- no CSRF on cookie flows (API is bearer-only today)
- no per-org rate limits
- PR approval UI covers plan and PR; deploy approvals are not implemented
- invite emails are plain text; no i18n
- search covers tasks only

## 8. Rollback

- application: roll back the image tag; migrations are forward-only and additive
- data: restore the last verified dump (`scripts/restore-check.sh`)
- git: revoke the pilot token and reconnect repos with a new one
