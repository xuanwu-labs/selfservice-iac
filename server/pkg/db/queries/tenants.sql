-- tenants queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetTenant :one
SELECT * FROM tenants
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTenantByLogicalId :one
SELECT * FROM tenants
WHERE tenant_logical_id = $1 AND deleted_at IS NULL;

-- name: ListTenants :many
SELECT * FROM tenants
WHERE deleted_at IS NULL
ORDER BY created_at;

-- name: CreateTenant :one
INSERT INTO tenants (id, tenant_logical_id, name, isolation_level, kind, owner_team_id, tag_namespace_json, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateTenant :one
UPDATE tenants
SET name = $2, isolation_level = $3, kind = $4, owner_team_id = $5, tag_namespace_json = $6, status = $7, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTenant :exec
UPDATE tenants
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
