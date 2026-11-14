-- +goose Up
ALTER TABLE projects ADD COLUMN policy_required_approvals int NOT NULL DEFAULT 1
    CHECK (policy_required_approvals BETWEEN 1 AND 3);

CREATE TABLE approval_votes (
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    approval_id uuid NOT NULL REFERENCES approvals(id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    decision    text NOT NULL CHECK (decision IN ('approved','rejected')),
    reason      text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (approval_id, user_id)
);
ALTER TABLE approval_votes ENABLE ROW LEVEL SECURITY;
CREATE POLICY approval_votes_tenant ON approval_votes
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE approval_votes;
ALTER TABLE projects DROP COLUMN policy_required_approvals;
