-- +goose Up
CREATE TYPE env_name AS ENUM ('dev','staging','prod');
CREATE TYPE deploy_status AS ENUM ('queued','running','succeeded','failed','rolled_back');

CREATE TABLE environments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       env_name NOT NULL,
    url        text,
    config     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

CREATE TABLE deployments (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    release_ref    text NOT NULL,
    status         deploy_status NOT NULL DEFAULT 'queued',
    deployed_by    uuid REFERENCES users(id),
    approved_by    uuid REFERENCES users(id),
    approved_at    timestamptz,
    started_at     timestamptz,
    finished_at    timestamptz,
    log            text NOT NULL DEFAULT ''
);

ALTER TABLE environments ENABLE ROW LEVEL SECURITY;
CREATE POLICY environments_tenant ON environments USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());
ALTER TABLE deployments ENABLE ROW LEVEL SECURITY;
CREATE POLICY deployments_tenant ON deployments USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE deployments, environments;
DROP TYPE deploy_status;
DROP TYPE env_name;
