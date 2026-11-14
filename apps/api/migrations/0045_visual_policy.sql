-- +goose Up
ALTER TABLE projects ADD COLUMN visual_threshold double precision NOT NULL DEFAULT 0.01;
ALTER TABLE projects ADD COLUMN visual_masks text NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE projects DROP COLUMN visual_masks;
ALTER TABLE projects DROP COLUMN visual_threshold;
