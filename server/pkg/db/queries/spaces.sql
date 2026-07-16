-- spaces queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetSpace :one
SELECT * FROM spaces
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListSpaces :many
SELECT * FROM spaces
WHERE deleted_at IS NULL
ORDER BY name;

-- name: ListSpacesByProject :many
SELECT * FROM spaces
WHERE project_id = $1 AND deleted_at IS NULL
ORDER BY name;

-- name: ListSpacesByLayer :many
SELECT * FROM spaces
WHERE layer_logical_id = $1 AND deleted_at IS NULL
ORDER BY name;

-- name: CreateSpace :one
INSERT INTO spaces (id, name, project_id, layer_logical_id, repo_path, tags_json)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateSpace :one
UPDATE spaces
SET name = $2, layer_logical_id = $3, repo_path = $4, tags_json = $5, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteSpace :exec
UPDATE spaces
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
