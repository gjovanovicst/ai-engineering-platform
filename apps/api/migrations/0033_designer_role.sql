-- +goose Up
ALTER TABLE memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE memberships ADD CONSTRAINT memberships_role_check
    CHECK (role IN ('owner','admin','designer','member','viewer'));

ALTER TABLE invites DROP CONSTRAINT invites_role_check;
ALTER TABLE invites ADD CONSTRAINT invites_role_check
    CHECK (role IN ('owner','admin','designer','member','viewer'));

ALTER TABLE sessions DROP CONSTRAINT sessions_role_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_role_check
    CHECK (role IN ('owner','admin','designer','member','viewer'));

-- +goose Down
ALTER TABLE sessions DROP CONSTRAINT sessions_role_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_role_check
    CHECK (role IN ('owner','admin','member','viewer'));
ALTER TABLE invites DROP CONSTRAINT invites_role_check;
ALTER TABLE invites ADD CONSTRAINT invites_role_check
    CHECK (role IN ('owner','admin','member','viewer'));
ALTER TABLE memberships DROP CONSTRAINT memberships_role_check;
ALTER TABLE memberships ADD CONSTRAINT memberships_role_check
    CHECK (role IN ('owner','admin','member','viewer'));
