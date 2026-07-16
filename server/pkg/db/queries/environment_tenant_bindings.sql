-- environment_tenant_bindings queries. ID is app-generated (snowflake), passed by caller.
-- Hard-delete only: table has no deleted_at column, so no soft-delete filtering.

-- name: GetBinding :one
SELECT * FROM environment_tenant_bindings
WHERE id = $1;

-- name: GetBindingByTriple :one
SELECT * FROM environment_tenant_bindings
WHERE env_id = $1 AND tenant_id = $2 AND layer_logical_id = $3;

-- name: ListBindingsByEnv :many
SELECT * FROM environment_tenant_bindings
WHERE env_id = $1
ORDER BY created_at;

-- name: ListBindingsByTenant :many
SELECT * FROM environment_tenant_bindings
WHERE tenant_id = $1
ORDER BY created_at;

-- name: CreateBinding :one
INSERT INTO environment_tenant_bindings (id, env_id, tenant_id, layer_logical_id, vpc_stack_id, subnet_blocks_json, security_group_base_id, override_cloud_account_id, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: DeleteBinding :exec
DELETE FROM environment_tenant_bindings
WHERE id = $1;
