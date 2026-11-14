-- +goose Up
CREATE TABLE blueprints (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name       text NOT NULL,
    version    int NOT NULL DEFAULT 1,
    spec       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name, version)
);
ALTER TABLE blueprints ENABLE ROW LEVEL SECURITY;
CREATE POLICY blueprints_tenant ON blueprints USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE blueprints;
