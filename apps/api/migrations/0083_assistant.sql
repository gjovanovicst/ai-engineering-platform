-- +goose Up
CREATE TABLE assistant_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
    title text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_conversations_org_user_idx ON assistant_conversations (org_id, user_id, updated_at DESC);

CREATE TABLE assistant_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES assistant_conversations(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role text NOT NULL,
    content text NOT NULL DEFAULT '',
    tool_name text NOT NULL DEFAULT '',
    tool_input jsonb,
    tool_output jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assistant_messages_conversation_idx ON assistant_messages (conversation_id, created_at);

-- +goose Down
DROP TABLE assistant_messages;
DROP TABLE assistant_conversations;
