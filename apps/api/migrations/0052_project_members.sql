-- +goose Up
CREATE TABLE project_members (
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('admin','designer','member','viewer')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX project_members_user_idx ON project_members (org_id, user_id);
ALTER TABLE project_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_members_tenant ON project_members
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE project_members;
