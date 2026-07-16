-- stacks queries. ID is app-generated (snowflake), passed by caller.
-- Lifecycle: no deleted_at. Stacks are immutable once applied; migration_status tracks lifecycle.

-- name: GetStack :one
SELECT * FROM stacks
WHERE id = $1;

-- name: GetStackByRepoPath :one
SELECT * FROM stacks
WHERE repo_path = $1;

-- name: ListStacks :many
SELECT * FROM stacks
ORDER BY created_at DESC;

-- name: ListStacksBySpace :many
SELECT * FROM stacks
WHERE space_id = $1
ORDER BY created_at DESC;

-- name: ListStacksByLayer :many
SELECT * FROM stacks
WHERE layer = $1
ORDER BY created_at DESC;

-- name: ListStacksByEnv :many
SELECT * FROM stacks
WHERE env = $1
ORDER BY created_at DESC;

-- name: CreateStack :one
INSERT INTO stacks (
    id, space_id, catalog_item_id, layer_logical_id, layer_rule_set_version_id,
    owner_team_id, layer, component, env, tenant_id, stack_id, repo_path, state_key,
    terramate_tags_json, state_backend_id, pinned_commit, migration_status, sunset_deadline, version
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19
)
RETURNING *;

-- name: UpdateStack :one
UPDATE stacks
SET
    space_id = $2,
    layer_logical_id = $3,
    layer_rule_set_version_id = $4,
    migration_status = $5,
    sunset_deadline = $6,
    pinned_commit = $7,
    state_backend_id = $8,
    terramate_tags_json = $9,
    version = $10,
    updated_at = now()
WHERE id = $1
RETURNING *;
