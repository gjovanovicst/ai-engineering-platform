-- +goose Up
CREATE TABLE model_prices (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider       text NOT NULL,
    model          text NOT NULL,
    input_per_1k   numeric(12,6) NOT NULL DEFAULT 0,
    output_per_1k  numeric(12,6) NOT NULL DEFAULT 0,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, provider, model)
);
ALTER TABLE model_prices ENABLE ROW LEVEL SECURITY;
CREATE POLICY model_prices_tenant ON model_prices USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE model_prices;
