-- +goose Up
ALTER TABLE merge_queue ADD COLUMN pull_request_id uuid REFERENCES pull_requests(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE merge_queue DROP COLUMN pull_request_id;
