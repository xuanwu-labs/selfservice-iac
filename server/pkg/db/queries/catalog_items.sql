-- catalog_items queries. ID is app-generated (snowflake), passed by caller.
-- Soft-delete aware: active filters use WHERE deleted_at IS NULL.

-- name: GetCatalogItem :one
SELECT * FROM catalog_items
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListCatalogItems :many
SELECT * FROM catalog_items
WHERE deleted_at IS NULL
ORDER BY display_name;

-- name: ListCatalogItemsByLayer :many
SELECT * FROM catalog_items
WHERE layer_logical_id = $1 AND deleted_at IS NULL
ORDER BY display_name;

-- name: ListCatalogItemsByOwner :many
SELECT * FROM catalog_items
WHERE owner_team_id = $1 AND deleted_at IS NULL
ORDER BY display_name;

-- name: ListVisibleCatalogItems :many
SELECT * FROM catalog_items
WHERE visibility_json @> $1::jsonb AND deleted_at IS NULL
ORDER BY display_name;

-- name: PublishCatalogItem :one
INSERT INTO catalog_items (
    id, module_version_id, display_name, description, category, status,
    form_schema_json, defaults_json, cardinality, instance_key,
    per_instance_fields_json, shared_fields_json, layer_logical_id, stack_grouping,
    owner_team_id, default_tags_json, user_allowed_tag_keys_json, visibility_json
)
VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18
)
RETURNING *;

-- name: UpdateCatalogItem :one
UPDATE catalog_items
SET
    display_name = $2,
    description = $3,
    category = $4,
    status = $5,
    form_schema_json = $6,
    defaults_json = $7,
    cardinality = $8,
    instance_key = $9,
    per_instance_fields_json = $10,
    shared_fields_json = $11,
    layer_logical_id = $12,
    stack_grouping = $13,
    owner_team_id = $14,
    default_tags_json = $15,
    user_allowed_tag_keys_json = $16,
    visibility_json = $17,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCatalogItem :exec
UPDATE catalog_items
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
