-- +goose Up
CREATE TABLE runs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id      uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id         uuid REFERENCES tasks(id) ON DELETE SET NULL,
    agent_type      text NOT NULL,
    status          text NOT NULL DEFAULT 'queued'
                    CHECK (status IN ('queued','running','awaiting_approval','succeeded','failed','canceled')),
    current_phase   text,
    model           text NOT NULL DEFAULT '',
    budget          jsonb NOT NULL DEFAULT '{}',
    spent           jsonb NOT NULL DEFAULT '{}',
    failure_code    text,
    failure_detail  text,
    cancel_requested_at timestamptz,
    started_at      timestamptz,
    finished_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX runs_org_created_idx ON runs (org_id, created_at DESC);
CREATE INDEX runs_project_created_idx ON runs (project_id, created_at DESC);
-- one active run per task (docs/03 §4)
CREATE UNIQUE INDEX one_active_run_per_task ON runs (task_id)
    WHERE status IN ('queued','running','awaiting_approval') AND task_id IS NOT NULL;

CREATE TABLE run_events (
    run_id     uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq        bigint NOT NULL,
    type       text NOT NULL,
    actor      text NOT NULL DEFAULT 'system' CHECK (actor IN ('agent','human','system')),
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, seq)
);

CREATE TABLE run_phases (
    run_id     uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    phase      text NOT NULL,
    attempt    int NOT NULL DEFAULT 1,
    state      text NOT NULL DEFAULT 'pending'
               CHECK (state IN ('pending','active','done','failed','skipped')),
    started_at timestamptz,
    ended_at   timestamptz,
    meta       jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY (run_id, phase, attempt)
);

ALTER TABLE runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY runs_tenant ON runs USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

ALTER TABLE run_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY run_events_tenant ON run_events USING (
    run_id IN (SELECT id FROM runs WHERE org_id = app_org_id())
) WITH CHECK (
    run_id IN (SELECT id FROM runs WHERE org_id = app_org_id())
);

ALTER TABLE run_phases ENABLE ROW LEVEL SECURITY;
CREATE POLICY run_phases_tenant ON run_phases USING (
    run_id IN (SELECT id FROM runs WHERE org_id = app_org_id())
) WITH CHECK (
    run_id IN (SELECT id FROM runs WHERE org_id = app_org_id())
);

-- +goose Down
DROP TABLE run_phases, run_events, runs;
