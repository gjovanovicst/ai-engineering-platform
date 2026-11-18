-- +goose Up
-- A fix run records what it fixes: the review findings (kind 'review') or a
-- failed CI check (kind 'ci', with the check log in fix_context).
ALTER TABLE runs ADD COLUMN fix_kind text NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN fix_context text;

-- +goose Down
ALTER TABLE runs DROP COLUMN fix_context;
ALTER TABLE runs DROP COLUMN fix_kind;
