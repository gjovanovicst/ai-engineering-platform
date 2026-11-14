-- +goose Up
CREATE TABLE preview_environments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         text NOT NULL DEFAULT 'preview',
    image        text NOT NULL,
    port         int NOT NULL CHECK (port > 0 AND port < 65536),
    status       text NOT NULL DEFAULT 'requested' CHECK (status IN ('requested','starting','running','destroying','destroyed','failed')),
    url          text NOT NULL DEFAULT '',
    container_id text NOT NULL DEFAULT '',
    error        text NOT NULL DEFAULT '',
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX preview_env_active_idx ON preview_environments (status, expires_at);
ALTER TABLE preview_environments ENABLE ROW LEVEL SECURITY;
CREATE POLICY preview_env_tenant ON preview_environments
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE preview_environments;
