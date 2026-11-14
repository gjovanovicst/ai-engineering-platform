-- +goose Up
CREATE TABLE project_components (
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    component_id uuid NOT NULL REFERENCES component_entries(id) ON DELETE CASCADE,
    mode         text NOT NULL DEFAULT 'allow' CHECK (mode IN ('allow','exception')),
    reason       text NOT NULL DEFAULT '',
    created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, component_id)
);
ALTER TABLE project_components ENABLE ROW LEVEL SECURITY;
CREATE POLICY project_components_tenant ON project_components
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE project_components;
