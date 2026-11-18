-- +goose Up
-- A fix run points at the development run whose pull request it updates. It
-- carries the same approved plan; the review findings are loaded from the
-- latest review of the task at execution time.
ALTER TABLE runs ADD COLUMN fix_of_run_id uuid REFERENCES runs (id);

-- +goose Down
ALTER TABLE runs DROP COLUMN fix_of_run_id;
