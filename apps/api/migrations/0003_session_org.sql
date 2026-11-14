-- +goose Up
-- Active organization and role belong to the session: a refresh keeps the
-- caller in the same org without re-reading memberships (docs/12-auth.md §4.1).
ALTER TABLE sessions
    ADD COLUMN org_id uuid REFERENCES orgs(id) ON DELETE SET NULL,
    ADD COLUMN role text CHECK (role IN ('owner','admin','member','viewer'));

-- +goose Down
ALTER TABLE sessions
    DROP COLUMN role,
    DROP COLUMN org_id;
