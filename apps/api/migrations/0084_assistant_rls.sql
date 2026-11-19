-- +goose Up
-- The assistant tables were created without RLS in 0083; every other org
-- scoped table has the tenant policy, so the app role could not read them.
ALTER TABLE assistant_conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY assistant_conversations_tenant ON assistant_conversations
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

ALTER TABLE assistant_messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY assistant_messages_tenant ON assistant_messages
    USING (org_id = app_org_id()) WITH CHECK (org_id = app_org_id());

-- +goose Down
DROP POLICY assistant_messages_tenant ON assistant_messages;
ALTER TABLE assistant_messages DISABLE ROW LEVEL SECURITY;
DROP POLICY assistant_conversations_tenant ON assistant_conversations;
ALTER TABLE assistant_conversations DISABLE ROW LEVEL SECURITY;
