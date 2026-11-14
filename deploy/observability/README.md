# Observability

What is collected today:

| Signal | Where |
|---|---|
| Traces (`run.execute`, `agent.execute`, HTTP) | OTLP from `aipd`/`aip-worker` when `AIP_OTEL_ENDPOINT` is set |
| Delivery metrics (lead time, approvals, rework) | `GET /orgs/{org}/metrics` (business view) |
| Run/approval state | Postgres (`runs`, `approvals`, `river_job`) |

The alert rules in `prometheus/rules.yaml` use two sources:

1. **sql_exporter** (`queries.yml`) for queue and failure signals — no custom
   application metrics needed, so the alerts work even before tracing is on.
2. **OTel collector spanmetrics** for HTTP 5xx and latency — requires the
   collector with the `spanmetrics` connector and a Prometheus exporter:

```yaml
# otel-collector.yaml (excerpt)
connectors:
  spanmetrics:
    dimensions:
      - name: http.status_code
service:
  pipelines:
    traces: { receivers: [otlp], exporters: [otlp, spanmetrics] }
    metrics: { receivers: [spanmetrics], exporters: [prometheus] }
```

Point Prometheus at the collector (`:8889` by default) and at sql_exporter.

## Wiring

```bash
# sql_exporter (binary or container) with:
#   AIP_DB_DSN=postgres://aip_app:...@postgres:5432/aip?sslmode=require
docker run -d --name sql-exporter -p 9399:9399 \
  -v $PWD/queries.yml:/queries.yml:ro \
  -e DATA_SOURCE_NAME="$AIP_DB_DSN" \
  --entrypoint /bin/sql_exporter burnyd/sql-exporter -config.file=/queries.yml
```

Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: aip-db
    static_configs: [{ targets: ["sql-exporter:9399"] }]
  - job_name: aip-otel
    static_configs: [{ targets: ["otel-collector:8889"] }]
  - job_name: aip-worker
    static_configs: [{ targets: ["aip-worker:9090"] }]
```

Grafana: import `grafana/dashboard.json` (datasource: Prometheus).

## SLOs (proposed, measured over 28 days)

| SLO | Target | Source |
|---|---|---|
| Runs leave the queue | 99% < 5 min | `aip_run_queued_oldest_seconds` |
| Runs succeed | ≥ 90% | `aip:run_failure_ratio_1h` |
| Approval turnaround | 90% < 24h | `aip_approval_pending_oldest_seconds` |
| API availability | 99.9% | spanmetrics 5xx ratio |

Error budget policy: when the 28-day budget is exhausted, feature work pauses
and the team works on reliability until the budget recovers.

## Alerts

`alerts.yml` contains API/worker availability, queue lag/depth and 5xx rules.
Scrape `aip-api:8080/metrics` and `aip-worker:9090/metrics`.

## Alert routing

`alertmanager.yml` routes critical alerts to the on-call receiver and warnings to
the team, with inhibitions so an API/worker outage suppresses its secondary
symptoms. Replace the `REPLACE_*` placeholders and keep credentials in the deploy
secret; no secret is committed here.

## Dashboard

`grafana-dashboard.json` (uid `aip-slo`) has API/worker up, 5xx rate, average
request duration, queue lag/depth and run counts by status. Import it into
Grafana and point the dashboard variable at the Prometheus datasource.

## Local stack (verified)

```bash
docker compose --profile observability up -d prometheus alertmanager grafana
```

- Prometheus `:19090` scrapes `api:8080/metrics` and `worker:9090/metrics`,
  evaluates `alerts.yml`, sends to Alertmanager.
- Alertmanager `:19093` uses `alertmanager.dev.yml` (webhook to a black hole).
- Grafana `:13000` (admin/admin) provisions the Prometheus datasource and the
  `aip-slo` dashboard.

Live drill: stopping the worker produced `AIPWorkerDown` `firing` in Prometheus
and `active` in Alertmanager; the worker was restarted and became healthy. The
drill exposed a duplicate-series bug (`up == 0 or absent(...)`); the rule now uses
`up{job="aip-worker"} == 0` only.

Not production: no persistent Prometheus volume, dev admin password, placeholder
receivers in the production Alertmanager config.
