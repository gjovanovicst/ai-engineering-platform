-- +goose Up
CREATE TABLE org_crypto (
    org_id     uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    kms_url    text NOT NULL,
    key_id     text NOT NULL,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE org_crypto ENABLE ROW LEVEL SECURITY;
CREATE POLICY org_crypto_tenant ON org_crypto
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE org_crypto;
