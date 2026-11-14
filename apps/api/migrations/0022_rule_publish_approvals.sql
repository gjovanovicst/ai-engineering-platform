-- +goose Up
ALTER TABLE approvals ADD COLUMN rule_version_id uuid REFERENCES rule_versions(id) ON DELETE CASCADE;
ALTER TABLE approvals ADD COLUMN requested_by uuid REFERENCES users(id);
CREATE INDEX approvals_rule_pending_idx ON approvals (org_id, rule_version_id) WHERE status = 'pending';

-- +goose Down
DROP INDEX approvals_rule_pending_idx;
ALTER TABLE approvals DROP COLUMN requested_by;
ALTER TABLE approvals DROP COLUMN rule_version_id;
