# 14 · Runbook

What to do when something breaks. Every step names the command and the signal
that tells you whether the fix worked. Commands assume the compose stack and the
`aip` database; replace hostnames for production.

## 1. Golden signals and alerts

| Signal | Source | Alert when |
|---|---|---|
| Oldest queued run | `runs.created_at` where status = `queued` | > 5 min |
| Failed run rate | `runs` status = `failed` | > 10% over 15 min |
| Run duration p95 | spans `run.execute` | > 20 min |
| HTTP 5xx | `http.status_code` span attribute / access log | > 1% over 5 min |
| Webhook rejections | `invalid_signature` responses / logs | > 5 in 5 min |
| Sandbox count | `docker ps --filter name=aip-sbx-` | > 20 for one worker |
| DB connections | `pg_stat_activity` | > 80% of `max_connections` |

Tracing is opt-in: set `AIP_OTEL_ENDPOINT` and every request/run/agent gets a
span (`run.execute`, `agent.execute`). Without it, use the SQL below.

Ready-made alerts, the sql_exporter queries behind them and a Grafana dashboard
live in [`deploy/observability/`](../deploy/observability/README.md).

## 2. Scenario: runs stay `queued`

1. Is the worker up? `docker compose ps` / `systemctl status aip-worker`.
2. Worker logs: `docker logs aip-worker --since 10m`. Look for
   `queue start`, `shutdown requested`, migration errors.
3. Queue depth and oldest job:

```sql
select kind, state, count(*), min(attempted_at) as oldest_attempt
from river_job group by 1,2 order by 3 desc;
```

4. If the worker is up but the queue is not moving, check the worker's DB
   connection (`AIP_DB_DSN`) and that River migrations exist (`aipctl migrate up`
   applies them).
5. A stuck `running` job with a dead worker is retried by River after its
   timeout; to force it: `update river_job set state='retryable' where id = ...`.

## 3. Scenario: a run is `running` forever

1. Find the sandbox: `docker ps --filter name=aip-sbx-`.
2. Inspect the agent command:
   `docker logs <aip-sbx-id>` is empty by design; the run log is authoritative:
   `GET /api/v1/orgs/{org}/runs/{run}/events`.
3. Cancel through the API (this stops the sandbox context):
   `POST /api/v1/orgs/{org}/runs/{run}/cancel`.
4. If the container survives cancellation, remove it:
   `docker rm -f <aip-sbx-id>`; the run is already `canceled`.
5. If the agent command itself hangs on a dependency, fix `AIP_AGENT_COMMAND` or
   the test command; both are configuration, not model output.

## 4. Scenario: the merge webhook never arrives

1. Check the git host delivery log (GitHub: Settings → Webhooks → Recent
   Deliveries; GitLab: Settings → Webhooks → Edit → Recent events).
2. Verify the URL: `GET /orgs/{org}/projects/{project}/repos` returns the exact
   `webhook_url` per repo; the secret shown there must match the git host.
3. A `401` means the signature/token is wrong: recreate the webhook in the git
   host with the repo secret (rotation: connect the repo again with a new
   secret).
4. A `404` means the repo id in the URL is wrong or the provider path does not
   match the connected provider.
5. If the merge happened but the task stayed in `review`, the PR/MR was not
   approved in the platform (`approval_required` in the delivery response) or the
   PR row is missing: check `select * from pull_requests where number = <n>`.
   Re-deliver the webhook after approving.

## 5. Scenario: sandbox containers leak

```bash
docker ps --filter name=aip-sbx- --format '{{.Names}} {{.Status}}'
docker rm -f $(docker ps -aq --filter name=aip-sbx-)   # only if no runs are active
```

Then check why: a worker killed with `SIGKILL` skips `Destroy`; run cleanup is
best-effort by design. The next deploy should use the K8s sandbox driver, where
the Job garbage collector handles this.

## 6. Scenario: 5xx spike after a deploy

1. `docker logs aipd --since 5m | grep '"status":5'`.
2. `/readyz` failing means the database is unreachable — check the pool and the
   DSN, not the code.
3. Roll back the binary (`docker compose up -d --no-deps aipd` with the previous
   tag). Migrations are forward-only: if the release added one, roll back the
   application first; the new columns are additive and safe to keep.
4. If RLS errors appear (`new row violates row-level security policy`), verify
   the connection user is `aip_app` (NOBYPASSRLS) and that the request set
   `app.org_id`; a missing setting returns no rows rather than an error by
   design.

## 7. Scenario: cost or duration regression

1. Compare spans `run.execute` by `agent.type` over the last 24h (or
   `GET /orgs/{org}/metrics?days=1` for lead time and rework).
2. High duration with low output usually means the agent command or the test
   command is waiting on a network dependency; check `agent.exit_code` spans.
3. Rework (`runs_per_task`) rising means plans are being rejected or tests fail
   after implementation; look at `runs.failure_code`:

```sql
select failure_code, count(*) from runs
where status = 'failed' and created_at > now() - interval '24 hours'
group by 1 order by 2 desc;
```

## 8. Security incident

| Situation | Action |
|---|---|
| Git token leaked | Revoke it at the provider, set a new `GITHUB_TOKEN`/`GITLAB_TOKEN`, restart the worker and API |
| JWT signing key leaked | Replace `jwt.pem` (`make keygen`) and restart; all access tokens become invalid; refresh tokens stay valid until expiry (rotate by revoking sessions) |
| User account compromise | Revoke the user's sessions (auth store `RevokeAllForUser`), rotate the password, review `audit_log` |
| Webhook secret leaked | Reconnect the repo with a new secret and update the git host |
| Suspicious run output | Cancel the run, inspect `run_events`, check the diff, and rotate the repo token if the sandbox had credentials |

`audit_log` is append-only; never update or delete rows there.

## 9. Backup and restore

Automation lives in the repository:

- `scripts/backup.sh` — `pg_dump -Fc --no-owner --no-privileges`, sha256
  checksum, retention (`RETENTION_DAYS`, default 14). Run with `make backup`.
- `deploy/k8s/backup-cronjob.yaml` — nightly CronJob (02:15) to a 20Gi PVC,
  using the same script from a ConfigMap.
- `scripts/restore-check.sh` — restores the newest dump into a scratch database
  and prints `orgs`, `runs` and migration counts; run monthly with
  `make restore-check`.

- Postgres: daily `pg_dump -Fc` plus WAL archiving in production; test a restore
  monthly.
- Restore into a fresh database:
  `pg_restore -d aip --no-owner --role=aip_migrator backup.dump`, then
  `aipctl migrate up` (no-op if the dump is current).
- Redis holds rate-limit counters and SSE fanout only; losing it is safe.

## 10. On-call checklist

- [ ] Confirm the blast radius: one run, one project, one org, or everything.
- [ ] Check the worker before the API; most run failures are queue/sandbox.
- [ ] Prefer cancel/retry over editing state by hand.
- [ ] Any manual SQL that changes state must be recorded in the incident notes.
- [ ] After the fix, add a test or an alert that would have caught it.
