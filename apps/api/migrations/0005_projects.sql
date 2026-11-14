-- +goose Up
CREATE TABLE projects (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name              text NOT NULL,
    slug              text NOT NULL,
    description       text NOT NULL DEFAULT '',
    blueprint_id      uuid,
    blueprint_version text,
    tech_stack        text[] NOT NULL DEFAULT '{}',
    status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    archived_at       timestamptz,
    module_map        jsonb NOT NULL DEFAULT '[]',
    test_gates        jsonb NOT NULL DEFAULT '{}',
    tool_policy       jsonb NOT NULL DEFAULT '{}',
    design_system_id  uuid,
    blueprint_sha256  bytea,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, slug)
);
CREATE INDEX projects_org_idx ON projects (org_id, status, updated_at DESC);

CREATE TRIGGER projects_set_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
CREATE POLICY projects_tenant ON projects
    USING (org_id = app_org_id())
    WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE projects;
