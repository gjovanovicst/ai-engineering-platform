-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION aip_audit_notify() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_notify('aip_audit', json_build_object(
        'id', NEW.id, 'org_id', NEW.org_id, 'actor_kind', NEW.actor_kind,
        'action', NEW.action, 'target', NEW.target, 'created_at', NEW.created_at,
        'hash', NEW.hash
    )::text);
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_log_notify AFTER INSERT ON audit_log
FOR EACH ROW EXECUTE FUNCTION aip_audit_notify();

-- +goose Down
DROP TRIGGER audit_log_notify ON audit_log;
DROP FUNCTION aip_audit_notify();
