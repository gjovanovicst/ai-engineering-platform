-- +goose Up
CREATE TABLE scim_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name       text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
ALTER TABLE scim_tokens ENABLE ROW LEVEL SECURITY;
CREATE POLICY scim_tokens_tenant ON scim_tokens
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE scim_tokens;
