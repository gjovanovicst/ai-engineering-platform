-- +goose Up
-- A project may override the global AIP_TEST_COMMAND (for example a Go API +
-- Node web repository needs both toolchains in one gate).
ALTER TABLE projects ADD COLUMN test_command text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN test_command;
