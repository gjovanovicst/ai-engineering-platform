-- +goose Up
ALTER TABLE runs ADD COLUMN plan jsonb, ADD COLUMN plan_sha256 text;

CREATE TABLE approvals (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id       uuid REFERENCES runs(id) ON DELETE CASCADE,
    type         text NOT NULL CHECK (type IN ('plan','pr','deploy_prod','rule_publish')),
    status       text NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','approved','rejected','changes_requested','expired')),
    requested_at timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz,
    decided_by   uuid REFERENCES users(id),
    reason       text,
    expires_at   timestamptz
);
CREATE INDEX approvals_pending_idx ON approvals (org_id, status, requested_at DESC);
CREATE UNIQUE INDEX one_pending_plan_approval_per_run ON approvals (run_id)
    WHERE status = 'pending' AND type = 'plan';

ALTER TABLE approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY approvals_tenant ON approvals USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE approvals;
ALTER TABLE runs DROP COLUMN plan_sha256, DROP COLUMN plan;
