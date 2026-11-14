-- +goose Up
CREATE TABLE usage_ledger (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id        uuid REFERENCES runs(id) ON DELETE SET NULL,
    provider      text NOT NULL,
    model         text NOT NULL,
    input_tokens  int NOT NULL DEFAULT 0,
    output_tokens int NOT NULL DEFAULT 0,
    cost_usd      numeric(12,6) NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX usage_ledger_org_idx ON usage_ledger (org_id, created_at DESC);
ALTER TABLE usage_ledger ENABLE ROW LEVEL SECURITY;
CREATE POLICY usage_ledger_tenant ON usage_ledger USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE usage_ledger;
