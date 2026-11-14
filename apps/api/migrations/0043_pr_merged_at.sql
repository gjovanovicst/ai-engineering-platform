-- +goose Up
ALTER TABLE pull_requests ADD COLUMN merged_at timestamptz;
UPDATE pull_requests SET merged_at = created_at WHERE state = 'merged';

-- +goose Down
ALTER TABLE pull_requests DROP COLUMN merged_at;
