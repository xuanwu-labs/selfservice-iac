-- layer_rule_set_versions queries. PK is version_id INTEGER (NOT snowflake), passed by caller.
-- Versioned-lifecycle table: uses status/superseded_at instead of deleted_at, so no soft-delete filtering.

-- name: GetRuleSetVersion :one
SELECT * FROM layer_rule_set_versions
WHERE version_id = $1;

-- name: GetActiveRuleSetVersion :one
SELECT * FROM layer_rule_set_versions
WHERE status = 'active' AND is_default = true;

-- name: GetDefaultRuleSetVersion :one
SELECT * FROM layer_rule_set_versions
WHERE is_default = true;

-- name: ListRuleSetVersions :many
SELECT * FROM layer_rule_set_versions
ORDER BY version_id DESC;

-- name: CreateRuleSetVersion :one
INSERT INTO layer_rule_set_versions (version_id, layers_json, status, is_default, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: SupersedeRuleSetVersion :exec
UPDATE layer_rule_set_versions
SET status = 'superseded', superseded_at = now(), superseded_by = $2
WHERE version_id = $1;
