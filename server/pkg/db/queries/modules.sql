-- modules queries. ID is app-generated (snowflake), passed by caller.
-- Status-based lifecycle (no deleted_at): status field tracks state, e.g. 'pending_validation'.

-- name: GetModule :one
SELECT * FROM modules
WHERE id = $1;

-- name: GetModuleByGitSource :one
SELECT * FROM modules
WHERE git_source = $1;

-- name: ListModules :many
SELECT * FROM modules
ORDER BY name;

-- name: ListModulesByOwner :many
SELECT * FROM modules
WHERE owner_team_id = $1
ORDER BY name;

-- name: ListModulesByLayer :many
SELECT * FROM modules
WHERE layer = $1
ORDER BY name;

-- name: CreateModule :one
INSERT INTO modules (id, name, git_source, provider, layer, owner_team_id, status, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateModule :one
UPDATE modules
SET name = $2, provider = $3, layer = $4, owner_team_id = $5, description = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateModuleStatus :one
UPDATE modules
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;
