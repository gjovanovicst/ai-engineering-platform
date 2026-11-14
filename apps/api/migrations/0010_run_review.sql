-- +goose Up
ALTER TABLE runs ADD COLUMN review jsonb;

-- +goose Down
ALTER TABLE runs DROP COLUMN review;
