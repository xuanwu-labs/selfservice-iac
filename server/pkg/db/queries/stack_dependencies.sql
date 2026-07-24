-- stack_dependencies queries. ID is app-generated (snowflake), passed by caller.
-- Hard-delete only: table has no deleted_at column, so no soft-delete filtering.

-- name: GetStackDependency :one
SELECT * FROM stack_dependencies
WHERE id = $1;

-- name: ListDependenciesByStack :many
SELECT * FROM stack_dependencies
WHERE from_stack_id = $1
ORDER BY created_at;

-- name: ListDependentsByStack :many
SELECT * FROM stack_dependencies
WHERE to_stack_id = $1
ORDER BY created_at;

-- name: CreateStackDependency :one
INSERT INTO stack_dependencies (id, from_stack_id, to_stack_id, kind, variable_name, output_key, inject_as, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeleteStackDependency :exec
DELETE FROM stack_dependencies
WHERE id = $1;
