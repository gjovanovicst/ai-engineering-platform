-- +goose Up
CREATE TABLE ai_provider_configs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name         text NOT NULL,
    base_url     text NOT NULL,
    model        text NOT NULL DEFAULT '',
    api_key_enc  bytea,
    key_hint     text,
    enabled      boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);
ALTER TABLE ai_provider_configs ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_provider_configs_tenant ON ai_provider_configs
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE ai_provider_configs;
