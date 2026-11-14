-- name: CreateAgentPlugin :one
INSERT INTO agent_plugins (org_id, name, version, image, command, agent_types, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, version, image, command, agent_types, created_at;

-- name: ListAgentPlugins :many
SELECT id, name, version, image, command, agent_types, created_at
FROM agent_plugins WHERE org_id = $1 ORDER BY created_at DESC;

-- name: SetProjectAgentPlugin :one
UPDATE projects SET agent_plugin_id = $3, updated_at = now()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: ProjectAgentPlugin :one
SELECT p.id, p.name, p.version, p.image, p.command, p.agent_types
FROM projects pr JOIN agent_plugins p ON p.id = pr.agent_plugin_id
WHERE pr.id = $1 AND pr.org_id = $2;
