-- +goose Up
-- 0001 granted ALL TABLES to aip_app, but that only covers the tables that
-- existed then; the assistant tables were created later and had no grant (the
-- RLS policy alone is not enough — the app role needs the table privileges).
GRANT SELECT, INSERT, UPDATE, DELETE ON assistant_conversations TO aip_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON assistant_messages TO aip_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE ON assistant_messages FROM aip_app;
REVOKE SELECT, INSERT, UPDATE, DELETE ON assistant_conversations FROM aip_app;
