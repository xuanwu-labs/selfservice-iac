-- environments queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetEnvironment :one
SELECT * FROM environments
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetEnvironmentByLogicalId :one
SELECT * FROM environments
WHERE env_logical_id = $1 AND deleted_at IS NULL;

-- name: ListEnvironments :many
SELECT * FROM environments
WHERE deleted_at IS NULL
ORDER BY created_at;

-- name: CreateEnvironment :one
INSERT INTO environments (id, env_logical_id, display_name, stage, cloud_account_id, region, network_topology, tag_namespace_json, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateEnvironment :one
UPDATE environments
SET display_name = $2, stage = $3, cloud_account_id = $4, region = $5, network_topology = $6, tag_namespace_json = $7, status = $8, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEnvironment :exec
UPDATE environments
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
