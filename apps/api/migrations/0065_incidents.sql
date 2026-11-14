-- +goose Up
CREATE TABLE incidents (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id  uuid REFERENCES projects(id) ON DELETE CASCADE,
    title       text NOT NULL,
    severity    text NOT NULL CHECK (severity IN ('sev1','sev2','sev3','sev4')),
    status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open','resolved')),
    opened_at   timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    created_by  uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX incidents_org_idx ON incidents (org_id, opened_at DESC);
ALTER TABLE incidents ENABLE ROW LEVEL SECURITY;
CREATE POLICY incidents_tenant ON incidents
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE incidents;
