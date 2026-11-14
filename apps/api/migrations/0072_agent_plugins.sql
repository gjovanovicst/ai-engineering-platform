-- +goose Up
CREATE TABLE agent_plugins (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        text NOT NULL,
    version     text NOT NULL,
    image       text NOT NULL,
    command     text NOT NULL,
    agent_types text[] NOT NULL DEFAULT '{}',
    created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name, version)
);
ALTER TABLE agent_plugins ENABLE ROW LEVEL SECURITY;
CREATE POLICY agent_plugins_tenant ON agent_plugins
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

ALTER TABLE projects ADD COLUMN agent_plugin_id uuid REFERENCES agent_plugins(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE projects DROP COLUMN agent_plugin_id;
DROP TABLE agent_plugins;
