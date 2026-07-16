-- tag_policies queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetTagPolicy :one
SELECT * FROM tag_policies
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTagPolicyByScope :one
SELECT * FROM tag_policies
WHERE scope_type = $1 AND scope_id = $2 AND deleted_at IS NULL;

-- name: ListTagPoliciesByScopeType :many
SELECT * FROM tag_policies
WHERE scope_type = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- name: CreateTagPolicy :one
INSERT INTO tag_policies (id, scope_type, scope_id, tag_namespace_json, mandatory_keys_json, user_allowed_tag_keys_json, version)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateTagPolicy :one
UPDATE tag_policies
SET tag_namespace_json = $2, mandatory_keys_json = $3, user_allowed_tag_keys_json = $4, version = $5, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTagPolicy :exec
UPDATE tag_policies
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
