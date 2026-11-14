-- +goose Up
ALTER TABLE task_attachments ADD COLUMN object_key text NOT NULL DEFAULT '';
ALTER TABLE task_attachments ALTER COLUMN data DROP NOT NULL;

-- +goose Down
ALTER TABLE task_attachments ALTER COLUMN data SET NOT NULL;
ALTER TABLE task_attachments DROP COLUMN object_key;
