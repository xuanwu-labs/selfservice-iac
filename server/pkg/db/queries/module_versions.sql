-- module_versions queries. ID is app-generated (snowflake), passed by caller.
-- Lifecycle: no deleted_at. is_current marks the active version per module.

-- name: GetModuleVersion :one
SELECT * FROM module_versions
WHERE id = $1;

-- name: GetModuleVersionByRef :one
SELECT * FROM module_versions
WHERE module_id = $1 AND version = $2;

-- name: GetCurrentModuleVersion :one
SELECT * FROM module_versions
WHERE module_id = $1 AND is_current = TRUE;

-- name: ListModuleVersions :many
SELECT * FROM module_versions
WHERE module_id = $1
ORDER BY created_at DESC;

-- name: CreateModuleVersion :one
INSERT INTO module_versions (id, module_id, version, commit_sha, providers_json, variables_contract_json, is_current)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UnsetOtherCurrentVersions :exec
UPDATE module_versions
SET is_current = FALSE
WHERE module_id = $1 AND is_current = TRUE;

-- name: SetCurrentModuleVersion :one
UPDATE module_versions
SET is_current = TRUE
WHERE id = $1
RETURNING *;
