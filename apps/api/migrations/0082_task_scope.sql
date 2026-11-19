-- +goose Up
-- The Definition of Ready fields a story needs beyond title and acceptance:
-- the files it touches, what is out of scope, the test that proves it and the
-- rollback. Arrays keep the entries structured without a child table.
ALTER TABLE tasks ADD COLUMN files text[] NOT NULL DEFAULT '{}';
ALTER TABLE tasks ADD COLUMN out_of_scope text[] NOT NULL DEFAULT '{}';
ALTER TABLE tasks ADD COLUMN test_plan text[] NOT NULL DEFAULT '{}';
ALTER TABLE tasks ADD COLUMN rollback text[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE tasks DROP COLUMN rollback;
ALTER TABLE tasks DROP COLUMN test_plan;
ALTER TABLE tasks DROP COLUMN out_of_scope;
ALTER TABLE tasks DROP COLUMN files;
