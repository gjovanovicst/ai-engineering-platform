-- +goose Up
-- acceptance_proof and review are artifacts the runner uploads today; the
-- check constraint never allowed them, so both uploads failed silently.
ALTER TABLE run_artifacts DROP CONSTRAINT run_artifacts_kind_check;
ALTER TABLE run_artifacts ADD CONSTRAINT run_artifacts_kind_check
    CHECK (kind = ANY (ARRAY['plan','diff','log','test_report','context_manifest','screenshot','acceptance_proof','review'])
           OR kind LIKE 'visual/%');

-- +goose Down
ALTER TABLE run_artifacts DROP CONSTRAINT run_artifacts_kind_check;
DELETE FROM run_artifacts WHERE kind IN ('acceptance_proof','review');
ALTER TABLE run_artifacts ADD CONSTRAINT run_artifacts_kind_check
    CHECK (kind = ANY (ARRAY['plan','diff','log','test_report','context_manifest','screenshot'])
           OR kind LIKE 'visual/%');
