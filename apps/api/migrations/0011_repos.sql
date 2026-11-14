-- +goose Up
CREATE TABLE repos (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    project_id     uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider       text NOT NULL CHECK (provider IN ('github','gitlab')),
    name           text NOT NULL,
    external_id    text NOT NULL DEFAULT '',
    default_branch text NOT NULL DEFAULT 'main',
    webhook_secret text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider, name)
);

CREATE TABLE pull_requests (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    repo_id    uuid NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    provider   text NOT NULL,
    number     int NOT NULL,
    url        text NOT NULL DEFAULT '',
    state      text NOT NULL DEFAULT 'open',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (repo_id, number)
);

ALTER TABLE repos ENABLE ROW LEVEL SECURITY;
CREATE POLICY repos_tenant ON repos USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

ALTER TABLE pull_requests ENABLE ROW LEVEL SECURITY;
CREATE POLICY pull_requests_tenant ON pull_requests USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- Webhooks arrive without a user session; the repo id is in the URL. This
-- function resolves the tenant so the handler can then use normal RLS.
CREATE FUNCTION app_repo_org(repo uuid) RETURNS uuid
    LANGUAGE sql STABLE SECURITY DEFINER
    SET search_path = public
AS $$ SELECT org_id FROM repos WHERE id = repo $$;

GRANT EXECUTE ON FUNCTION app_repo_org(uuid) TO aip_app;

-- +goose Down
DROP FUNCTION app_repo_org(uuid);
DROP TABLE pull_requests, repos;
