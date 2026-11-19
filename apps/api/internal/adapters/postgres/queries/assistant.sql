-- name: CreateAssistantConversation :one
INSERT INTO assistant_conversations (org_id, user_id, project_id, title)
VALUES ($1, $2, $3, $4)
RETURNING id, org_id, user_id, project_id, title, created_at, updated_at;

-- name: GetAssistantConversation :one
SELECT id, org_id, user_id, project_id, title, created_at, updated_at
FROM assistant_conversations WHERE id = $1 AND org_id = $2;

-- name: ListAssistantConversations :many
SELECT id, org_id, user_id, project_id, title, created_at, updated_at
FROM assistant_conversations
WHERE org_id = $1 AND user_id = $2
ORDER BY updated_at DESC LIMIT $3;

-- name: TouchAssistantConversation :exec
UPDATE assistant_conversations SET updated_at = now() WHERE id = $1 AND org_id = $2;

-- name: AppendAssistantMessage :one
INSERT INTO assistant_messages (conversation_id, org_id, role, content, tool_name, tool_input, tool_output)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, conversation_id, org_id, role, content, tool_name, tool_input, tool_output, created_at;

-- name: ListAssistantMessages :many
SELECT id, conversation_id, org_id, role, content, tool_name, tool_input, tool_output, created_at
FROM assistant_messages WHERE conversation_id = $1 AND org_id = $2
ORDER BY created_at ASC LIMIT $3;
