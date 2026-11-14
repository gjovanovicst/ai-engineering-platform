-- +goose Up
CREATE TABLE org_sso (
    org_id          uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    entity_id       text NOT NULL,
    metadata_url    text NOT NULL,
    certificate_pem text NOT NULL DEFAULT '',
    enabled         boolean NOT NULL DEFAULT false,
    updated_at      timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE org_sso ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_sso_tenant ON org_sso
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE org_sso;
