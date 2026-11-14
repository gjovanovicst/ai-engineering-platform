# Operations runbook

Status: **2026-10-27** · owner: platform
Scope: deploy, upgrade, rollback, incident basics for the self-hosted stack.

## Topology

- `aipd` (API): stateless, behind a load balancer; N replicas are safe.
- `aip-worker`: runs jobs; scale replicas for throughput; one Docker socket per
  host; preview containers live on the worker host.
- Postgres 17 (RLS), Redis (rate limits/events), S3/MinIO (artifacts,
  attachments, baselines, embedding cache).
- Metrics: API `/metrics`, worker `AIP_WORKER_METRICS_ADDR` (default `:9090`).
  Alerts: `deploy/observability/alerts.yml`.

## Deploy

1. Build one image (`docker build -f deploy/api/Dockerfile`) and use it for API
   and worker; deploy the **same digest**. Verify it first:
   `IMAGE=ghcr.io/org/repo/api@sha256:... AIP_COSIGN_IDENTITY_REGEXP='^https://github.com/org/repo/' make verify-image`.
2. Run `aipctl migrate up` (or the compose `migrate` service) **before** starting
   the new API/worker.
3. Start API replicas, then worker replicas.
4. Check `/healthz`, `/metrics`, and one real run.

## Upgrade

- Migrations are forward-only; write them so old API/worker keep working during
  the rollout (expand/contract).
- Deploy DB migration first, then API, then worker.
- Never run two different worker image digests that expect different job payload
  shapes without a compatibility window.

## Rollback

- Application: redeploy the previous image digest. The DB is **not** rolled back
  automatically (down migrations are for development only; production restores
  from backup, see docs/29).
- If a migration is incompatible, stop workers, restore the last verified dump,
  then redeploy the previous image.
- Preview containers: `docker rm -f` by `aip-prev-*` label if the worker is gone.

## Incidents

| Symptom | First check |
|---|---|
| API 5xx | `/metrics` 5xx rate, API logs, Postgres health |
| Runs not starting | worker up, queue metrics (`aip_queue_jobs`), River job errors |
| Queue lag | worker count, provider latency, run budget |
| Checks not updating | checksync logs, GitHub token/app installation |
| Preview stuck | `docker ps`, `preview_environments.status`, worker logs |
| Audit doubt | `select aip_audit_broken_count()`; restore from backup if >0 |

## SLOs (provisional, measured where possible)

| SLO | Target | Source |
|---|---|---|
| API availability | 99.5% monthly | `aip_api_up`, 5xx |
| Run start latency (p95) | < 60 s | queue lag + run timestamps |
| Merge lead time (p50) | < 2 h | analytics |
| Restore RTO | < 1 h | backup drill |
| RPO | ≤ 24 h | daily backup |

These are targets, not measured guarantees. The Grafana dashboard is
`deploy/observability/grafana-dashboard.json`; alert rules include 99.5%
availability burn-rate (14.4x/2m critical, 6x/30m warning) and average latency
> 1s. Alertmanager routing is in `deploy/observability/alertmanager.yml`
(critical → on-call, warning → team, API/worker outages inhibit secondary noise).

## TODO before enterprise sale

- HA Postgres + tested failover
- Alertmanager routing/on-call rota
- OTel trace link from the run page (trace id not persisted yet)
- legal hold/retention automation
