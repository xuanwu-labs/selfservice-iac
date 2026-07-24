-- projects queries. ID is app-generated (snowflake), passed by caller.
-- projects table has no slug/status/tags_json columns (only id/name/team_id/timestamps).
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetProjectByName :one
SELECT * FROM projects
WHERE name = $1 AND deleted_at IS NULL;

-- name: ListProjects :many
SELECT * FROM projects
WHERE deleted_at IS NULL
ORDER BY name;

-- name: ListProjectsByTeam :many
SELECT * FROM projects
WHERE team_id = $1 AND deleted_at IS NULL
ORDER BY name;

-- name: CreateProject :one
INSERT INTO projects (id, name, team_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProject :exec
UPDATE projects
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
