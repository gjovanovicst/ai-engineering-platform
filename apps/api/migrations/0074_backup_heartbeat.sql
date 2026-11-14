-- +goose Up
CREATE TABLE backup_heartbeat (
    kind            text PRIMARY KEY,
    last_success_at timestamptz NOT NULL,
    detail          text NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE backup_heartbeat;
