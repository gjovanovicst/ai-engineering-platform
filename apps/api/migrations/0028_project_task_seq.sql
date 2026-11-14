-- +goose Up
ALTER TABLE projects ADD COLUMN task_seq bigint NOT NULL DEFAULT 0;
UPDATE projects p
SET task_seq = COALESCE((
    SELECT MAX(NULLIF(regexp_replace(t.ref, '\D', '', 'g'), '')::bigint)
    FROM tasks t WHERE t.project_id = p.id
), 0);

-- +goose Down
ALTER TABLE projects DROP COLUMN task_seq;
