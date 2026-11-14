# 16 · Metric definitions

A number without a definition is an opinion. Every metric below states its
formula, window, source and known bias. The API returns
`{"value", "unit", "source"}` with `source ∈ {measured, estimated, na}`.

Rules:

1. **Definition before number.** A metric ships together with its formula here.
2. **Never fake a value.** If the data does not exist, `source: "na"` — not `0`.
3. **One window per claim.** "40 minutes" is meaningless without 30d/28d/p90.
4. Time is UTC; durations are wall-clock, not business hours.
5. Canceled runs are excluded from success ratios but counted in throughput.

## 1. Lead time

**Question:** how long does one agent execution take?

```
lead_time(run) = finished_at - started_at        -- succeeded runs only
dashboard      = mean(lead_time) over the window, in minutes
```

- Source: measured (`runs`).
- Excludes: queued time (that is the queue metric), canceled/failed runs.
- Bias: a run that hangs until timeout inflates the mean; use p95 for alerting.
- SQL (`deploy/observability/queries.yml` equivalent):

```sql
SELECT avg(extract(epoch FROM (finished_at - started_at)))
FROM runs
WHERE org_id = $1 AND status = 'succeeded'
  AND created_at > now() - interval '30 days';
```

## 2. Approval time

**Question:** how long does a human take to approve a plan or a PR?

```
approval_time = decided_at - requested_at     -- approvals with status = approved
dashboard     = mean over the window, in minutes
```

- Source: measured (`approvals`).
- Excludes: rejected approvals and still-pending ones (report the oldest pending
  separately: `aip_approval_pending_oldest_seconds`).
- Bias: approvals created before a holiday inflate the mean; p90 is the useful
  number for "is the human the bottleneck?".

## 3. Runs per task (rework)

**Question:** how often does one task need another agent attempt?

```
runs_per_task = count(runs) / count(distinct task_id)   -- all runs of the org
```

- Source: measured.
- A task that needed plan → dev → review has 3 runs by design; compare against a
  baseline, not against 1. The rework signal is **development runs per task >
  1**: a second implementation run means the first was rejected or failed.
- Better query for rework:

```sql
SELECT avg(c) FROM (
  SELECT count(*) AS c FROM runs
  WHERE agent_type = 'development' AND org_id = $1
  GROUP BY task_id
) per_task;
```

## 4. Merged pull/merge requests

```
merged_prs = count(pull_requests) WHERE state = 'merged' AND created_at > window
```

- Source: measured.
- Depends on the git host calling the webhook; a missing webhook under-reports.
  Cross-check with the provider's own count when the numbers disagree.

## 5. Success rate

```
success_rate = succeeded / (succeeded + failed)     -- canceled excluded
```

- Source: measured.
- Use 15m for alerts (`aip_run_failed_ratio_15m`) and 28d for SLOs.

## 6. Cost per task

```
cost_usd(task) = sum over runs of (
    input_tokens  * price_input(provider, model)
  + output_tokens * price_output(provider, model)
)
```

- Source: **na** until `usage_ledger` is populated by the provider adapters.
- Never infer cost from wall-clock time.
- Until then, the API returns `value: 0, source: "na"` and dashboards must show
  "not collected", not a zero.

## 7. Slice-gate metrics (week 8)

These decide continue/pivot; they are defined here so the decision cannot drift.

| Metric | Definition | Target | Source |
|---|---|---|---|
| Merges without rework | share of merged PRs where the task had exactly **one succeeded development run** before merge | ≥ 60% | measured (`runs`, `pull_requests`) |
| Time per task | `done_at - created_at`, **median** over tasks done in the window | ≤ 40 min | measured (`tasks`) |
| Cost per task | §6, mean over the window | ≤ $5 | na today |
| "Would miss it" | share answering "very disappointed" if the product disappeared; ≥ 10 respondents | ≥ 60% | survey |

SQL for the first two:

```sql
-- merges without rework
SELECT
  count(*) FILTER (WHERE dev_runs = 1)::float / NULLIF(count(*), 0) AS share
FROM (
  SELECT pr.id,
         (SELECT count(*) FROM runs r
          WHERE r.task_id = pr.task_id AND r.agent_type = 'development'
            AND r.status = 'succeeded') AS dev_runs
  FROM pull_requests pr WHERE pr.state = 'merged'
) merged;

-- median time per task
SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY extract(epoch FROM (done_at - created_at)))
FROM tasks WHERE status = 'done' AND done_at > now() - interval '8 weeks';
```

## 8. What we deliberately do not measure

- Lines of code, commits per day, tokens per developer — vanity metrics that
  reward volume, not outcomes.
- "Agent utilisation" — the platform is not a seat product.
- Story points — the agent has no velocity in points.

## 9. Reporting cadence

- **Live:** dashboard (`deploy/observability/grafana/dashboard.json`).
- **Weekly:** lead time, approval time, rework, merged PRs.
- **Monthly:** restore drill, SLO review, error budget.
- **Week 8:** slice gate with §7 definitions and the raw queries attached.

## 8. Success metrics (implemented 2026-11-05)

Exposed in `GET /metrics` and Analytics:

| Metric | Definition | Source |
|---|---|---|
| `first_pass_merge_percent` | merged PRs whose task had exactly one succeeded development run | measured |
| `proof_validity_percent` | succeeded development runs with a non-null `acceptance_proof` | measured |
| `dev_runs_per_task` | average development runs per task in the window | measured |
| `escaped_defects_per_100_merges` | incidents / merged PRs × 100 | measured **proxy**; incidents are project-scoped, not linked to a specific merge |

Live demo snapshot: first-pass 100%, proof validity 22.22%, dev runs/task 1.41,
escaped 25/100. The escaped-defect number is a proxy until incidents carry a
merge/PR link.
