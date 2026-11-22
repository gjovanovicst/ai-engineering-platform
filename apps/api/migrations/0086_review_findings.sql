-- +goose Up
-- The review's findings had no lifecycle: a run stored the JSON, the panel
-- rendered it, and nothing recorded that a blocker was fixed or dismissed. The
-- merge could proceed with an open blocker, and a nit came back every review.
CREATE TABLE review_findings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    severity text NOT NULL DEFAULT 'nit',
    file text NOT NULL DEFAULT '',
    line integer NOT NULL DEFAULT 0,
    message text NOT NULL,
    status text NOT NULL DEFAULT 'open',
    decided_by uuid REFERENCES users(id) ON DELETE SET NULL,
    reason text NOT NULL DEFAULT '',
    fixed_by_run uuid REFERENCES runs(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX review_findings_task_idx ON review_findings (org_id, task_id, status);
CREATE INDEX review_findings_open_blockers_idx ON review_findings (org_id, task_id) WHERE status = 'open' AND severity = 'blocker';

ALTER TABLE review_findings ENABLE ROW LEVEL SECURITY;
CREATE POLICY review_findings_tenant ON review_findings
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON review_findings TO aip_app;

-- +goose Down
DROP TABLE review_findings;
