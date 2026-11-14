-- +goose Up
ALTER TABLE projects ADD COLUMN mcp_servers text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE projects DROP COLUMN mcp_servers;
