-- +goose Up
CREATE TABLE run_artifacts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    run_id     uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('plan','diff','log','test_report','context_manifest','screenshot')),
    bucket_key text NOT NULL,
    sha256     bytea NOT NULL,
    bytes      bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, kind)
);

ALTER TABLE run_artifacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY run_artifacts_tenant ON run_artifacts USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP TABLE run_artifacts;
