-- +goose Up
ALTER TABLE approvals ADD COLUMN pull_request_id uuid REFERENCES pull_requests(id) ON DELETE CASCADE;
CREATE INDEX approvals_pull_request_idx ON approvals (pull_request_id) WHERE pull_request_id IS NOT NULL;

-- +goose Down
DROP INDEX approvals_pull_request_idx;
ALTER TABLE approvals DROP COLUMN pull_request_id;
