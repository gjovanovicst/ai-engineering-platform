-- +goose Up
CREATE TABLE audit_witnesses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    object_key text NOT NULL,
    row_count  bigint NOT NULL,
    last_id    bigint NOT NULL,
    last_hash  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_witnesses_org_idx ON audit_witnesses (org_id, created_at DESC);
ALTER TABLE audit_witnesses ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_witnesses_tenant ON audit_witnesses
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE audit_witnesses;
