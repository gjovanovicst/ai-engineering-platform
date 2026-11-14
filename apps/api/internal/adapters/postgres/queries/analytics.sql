-- name: CountRunsByStatus :many
SELECT status, count(*)::bigint AS total
FROM runs
WHERE org_id = $1 AND created_at > now() - make_interval(days => $2::int)
GROUP BY status;

-- name: RunDurationStats :one
SELECT count(*)::bigint AS total,
       COALESCE(avg(extract(epoch FROM (finished_at - started_at))), 0)::float8 AS avg_seconds
FROM runs
WHERE org_id = $1 AND status = 'succeeded' AND started_at IS NOT NULL AND finished_at IS NOT NULL
  AND created_at > now() - make_interval(days => $2::int);

-- name: ApprovalDecisionStats :one
SELECT count(*)::bigint AS total,
       COALESCE(avg(extract(epoch FROM (decided_at - requested_at))), 0)::float8 AS avg_seconds
FROM approvals
WHERE org_id = $1 AND status = 'approved' AND decided_at IS NOT NULL
  AND requested_at > now() - make_interval(days => $2::int);

-- name: RunsPerTaskStats :one
SELECT COALESCE(avg(c), 0)::float8 AS avg_runs
FROM (SELECT count(*)::float8 AS c FROM runs WHERE org_id = $1 GROUP BY task_id) per_task;

-- name: PullRequestsMerged :one
SELECT count(*)::bigint AS total
FROM pull_requests
WHERE org_id = $1 AND state = 'merged'
  AND created_at > now() - make_interval(days => $2::int);

-- name: UsageCost :one
SELECT count(*)::bigint AS rows, COALESCE(sum(cost_usd), 0)::float8 AS cost
FROM usage_ledger
WHERE org_id = $1 AND created_at > now() - make_interval(days => $2::int);

-- name: OnboardingSeconds :one
SELECT COALESCE(avg(EXTRACT(EPOCH FROM (first.merged_at - projects.created_at))), 0)::float8 AS avg_seconds,
       count(*)::bigint AS projects
FROM projects
JOIN LATERAL (
    SELECT min(pr.merged_at) AS merged_at
    FROM pull_requests pr
    JOIN tasks t ON t.id = pr.task_id
    WHERE t.project_id = projects.id AND pr.state = 'merged' AND pr.merged_at IS NOT NULL
) first ON true
WHERE projects.org_id = $1
  AND first.merged_at > now() - make_interval(days => $2::int);

-- name: MergedPerDay :many
SELECT to_char(date_trunc('day', merged_at), 'YYYY-MM-DD') AS day, count(*)::bigint AS total
FROM pull_requests
WHERE org_id = $1 AND state = 'merged' AND merged_at > now() - make_interval(days => $2::int)
GROUP BY 1 ORDER BY 1;

-- name: LeadTimePerDay :many
WITH first_run AS (
    SELECT t.id AS task_id, min(r.started_at) AS started_at
    FROM tasks t
    JOIN runs r ON r.task_id = t.id AND r.agent_type = 'development' AND r.started_at IS NOT NULL
    GROUP BY t.id
)
SELECT to_char(date_trunc('day', pr.merged_at), 'YYYY-MM-DD') AS day,
       (avg(extract(epoch FROM (pr.merged_at - fr.started_at))) / 60)::float8 AS minutes
FROM pull_requests pr
JOIN first_run fr ON fr.task_id = pr.task_id
WHERE pr.org_id = $1 AND pr.state = 'merged' AND pr.merged_at > now() - make_interval(days => $2::int)
GROUP BY 1 ORDER BY 1;

-- name: IncidentCountWindow :one
SELECT count(*)::bigint FROM incidents
WHERE org_id = $1 AND opened_at > now() - make_interval(days => $2::int);

-- name: IncidentMTTRSeconds :one
SELECT COALESCE(avg(extract(epoch FROM (resolved_at - opened_at))), 0)::float8, count(*)::bigint
FROM incidents
WHERE org_id = $1 AND resolved_at IS NOT NULL AND opened_at > now() - make_interval(days => $2::int);

-- name: ProjectDORA :many
SELECT p.id, p.name,
       (SELECT count(*) FROM pull_requests pr
         JOIN tasks t ON t.id = pr.task_id
         WHERE t.project_id = p.id AND pr.state = 'merged' AND pr.merged_at > now() - make_interval(days => $2::int))::bigint AS merges,
       (SELECT count(*) FROM incidents i
         WHERE i.project_id = p.id AND i.opened_at > now() - make_interval(days => $2::int))::bigint AS incidents,
       COALESCE((SELECT avg(extract(epoch FROM (i.resolved_at - i.opened_at)))
         FROM incidents i
         WHERE i.project_id = p.id AND i.resolved_at IS NOT NULL AND i.opened_at > now() - make_interval(days => $2::int)), 0)::float8 AS mttr_seconds,
       p.created_at,
       (SELECT min(t.created_at) FROM tasks t WHERE t.project_id = p.id)::timestamptz AS first_task_at,
       (SELECT min(r.created_at) FROM runs r JOIN tasks t ON t.id = r.task_id WHERE t.project_id = p.id)::timestamptz AS first_run_at,
       (SELECT min(pr.created_at) FROM pull_requests pr JOIN tasks t ON t.id = pr.task_id WHERE t.project_id = p.id)::timestamptz AS first_pr_at,
       (SELECT min(pr.merged_at) FROM pull_requests pr
         JOIN tasks t ON t.id = pr.task_id
         WHERE t.project_id = p.id AND pr.state = 'merged' AND pr.merged_at IS NOT NULL)::timestamptz AS first_merged_at
FROM projects p
WHERE p.org_id = $1
ORDER BY p.name;

-- name: FirstPassMergeRate :one
SELECT COALESCE((count(*) FILTER (WHERE dev_runs = 1))::float8 / NULLIF(count(*), 0), 0)::float8 AS share
FROM (
    SELECT pr.id,
           (SELECT count(*) FROM runs r
             WHERE r.task_id = pr.task_id AND r.agent_type = 'development' AND r.status = 'succeeded') AS dev_runs
    FROM pull_requests pr
    WHERE pr.org_id = $1 AND pr.state = 'merged'
      AND pr.merged_at > now() - make_interval(days => $2::int)
) merged;

-- name: ProofValidity :one
SELECT COALESCE((count(*) FILTER (WHERE acceptance_proof IS NOT NULL))::float8 / NULLIF(count(*), 0), 0)::float8 AS share,
       count(*)::bigint AS total
FROM runs
WHERE org_id = $1 AND agent_type = 'development' AND status = 'succeeded'
  AND created_at > now() - make_interval(days => $2::int);

-- name: DevRunsPerTask :one
SELECT COALESCE(avg(c), 0)::float8 AS avg_runs
FROM (
    SELECT count(*)::float8 AS c FROM runs
    WHERE org_id = $1 AND agent_type = 'development'
      AND created_at > now() - make_interval(days => $2::int)
    GROUP BY task_id
) per_task;
