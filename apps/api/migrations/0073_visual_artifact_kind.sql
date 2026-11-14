-- +goose Up
ALTER TABLE run_artifacts DROP CONSTRAINT run_artifacts_kind_check;
ALTER TABLE run_artifacts ADD CONSTRAINT run_artifacts_kind_check
    CHECK (kind = ANY (ARRAY['plan','diff','log','test_report','context_manifest','screenshot'])
           OR kind LIKE 'visual/%');

-- +goose Down
ALTER TABLE run_artifacts DROP CONSTRAINT run_artifacts_kind_check;
ALTER TABLE run_artifacts ADD CONSTRAINT run_artifacts_kind_check
    CHECK (kind = ANY (ARRAY['plan','diff','log','test_report','context_manifest','screenshot']));
