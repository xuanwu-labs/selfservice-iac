-- layer_logical_refs queries. PK is logical_id TEXT (NOT snowflake), passed by caller.
-- Hard-lifecycle only: table has no deleted_at column, so no soft-delete filtering.

-- name: GetLayerLogicalRef :one
SELECT * FROM layer_logical_refs
WHERE logical_id = $1;

-- name: ListLayerLogicalRefs :many
SELECT * FROM layer_logical_refs
ORDER BY created_at;

-- name: CreateLayerLogicalRef :one
INSERT INTO layer_logical_refs (logical_id, current_display_name, notes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateLayerLogicalRefDisplayName :one
UPDATE layer_logical_refs
SET current_display_name = $2
WHERE logical_id = $1
RETURNING *;
