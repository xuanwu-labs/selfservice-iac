-- teams queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetTeam :one
SELECT * FROM teams
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTeamBySlug :one
SELECT * FROM teams
WHERE slug = $1 AND deleted_at IS NULL;

-- name: ListTeams :many
SELECT * FROM teams
WHERE deleted_at IS NULL
ORDER BY name;

-- name: ListTeamsByKind :many
SELECT * FROM teams
WHERE kind = $1 AND deleted_at IS NULL
ORDER BY name;

-- name: CreateTeam :one
INSERT INTO teams (id, name, slug, kind, status, tags_json, policy_json)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateTeam :one
UPDATE teams
SET name = $2, tags_json = $3, policy_json = $4, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTeam :exec
UPDATE teams
SET deleted_at = now(), status = 'deprecated'
WHERE id = $1 AND deleted_at IS NULL;
