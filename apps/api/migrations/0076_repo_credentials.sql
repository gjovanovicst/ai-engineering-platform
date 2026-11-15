-- +goose Up
-- Per-repo git credentials. Resolution order is: repo credential -> GitHub App
-- installation token -> global AIP_GITHUB_TOKEN, so a project can use a
-- fine-grained PAT scoped to its own repository instead of a shared token.
CREATE TABLE repo_credentials (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    repo_id    uuid NOT NULL UNIQUE REFERENCES repos(id) ON DELETE CASCADE,
    -- Ciphertext from AIP_SECRETS_KEY; the plaintext token never hits the DB.
    ciphertext bytea NOT NULL,
    hint       text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE repo_credentials ENABLE ROW LEVEL SECURITY;
CREATE POLICY repo_credentials_tenant ON repo_credentials
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- Explicit grant: a migration applied by a role other than aip_migrator does
-- not inherit the default privileges from deploy/postgres/00_roles.sql.
GRANT SELECT, INSERT, UPDATE, DELETE ON repo_credentials TO aip_app;

-- +goose Down
DROP TABLE repo_credentials;
