-- module_dependencies queries. ID is app-generated (snowflake), passed by caller.
-- Lifecycle: no deleted_at. Cascade-deleted with parent module_version (FK ON DELETE CASCADE).

-- name: GetModuleDependency :one
SELECT * FROM module_dependencies
WHERE id = $1;

-- name: ListDependenciesByVersion :many
SELECT * FROM module_dependencies
WHERE module_version_id = $1
ORDER BY variable_name;

-- name: CreateModuleDependency :one
INSERT INTO module_dependencies (id, module_version_id, variable_name, depends_on_layer, depends_on_module, output_key, required, description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;
