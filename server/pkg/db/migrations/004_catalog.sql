-- 004_catalog.sql: catalog_items — the service catalog (design.md §03 A3).
-- A catalog item binds a module version to a user-facing request form with
-- cardinality (D25), layer identity (D24), tag defaults (L6) and visibility.
-- layer_logical_id FK is back-filled by 010_layers (refs table created there).

-- +goose Up
CREATE TABLE IF NOT EXISTS catalog_items (
    id                          BIGINT       PRIMARY KEY,                     -- snowflake ID
    module_version_id           BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE RESTRICT,  -- pinned module version
    display_name                TEXT         NOT NULL,                         -- user-facing name (e.g. "RDS MySQL")
    description                 TEXT         NOT NULL DEFAULT '',              -- human-readable description
    category                    TEXT         NOT NULL DEFAULT '',              -- grouping (e.g. "database", "network")
    status                      TEXT         NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'active', 'deprecated', 'archived', 'blocked')),  -- proto CatalogItemStatus
    form_schema_json            JSONB        NOT NULL DEFAULT '{}',            -- frontend form schema (field definitions)
    defaults_json               JSONB        NOT NULL DEFAULT '{}',            -- S2 catalog defaults (doc 08 param pipeline)
    cardinality                 TEXT         NOT NULL DEFAULT 'single'
                                CHECK (cardinality IN ('single', 'list', 'map')),  -- D25: how codegen injects for_each/count
    instance_key                TEXT         NOT NULL DEFAULT '',              -- D25: variable name used as for_each key
    per_instance_fields_json    JSONB        NOT NULL DEFAULT '{}',            -- D25: fields that vary per instance (from each.value)
    shared_fields_json          JSONB        NOT NULL DEFAULT '{}',            -- D25: fields shared across all instances
    layer_logical_id            TEXT         NULL,                             -- FK added by 010_layers; layer this catalog item belongs to
    stack_grouping              TEXT         NOT NULL DEFAULT 'per-component'
                                CHECK (stack_grouping IN ('per-component', 'per-bundle', 'per-team', 'custom')),  -- D24 StackGranularity
    owner_team_id               BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,  -- team responsible for this catalog item
    default_tags_json           JSONB        NOT NULL DEFAULT '{}',            -- L6 catalog default tags (doc 08 tag 7-layer model)
    user_allowed_tag_keys_json  JSONB        NOT NULL DEFAULT '[]',            -- L7 user tag whitelist (doc 08; empty = no custom tags allowed)
    visibility_json             JSONB        NOT NULL DEFAULT '[]',            -- team_ids that can see/request this item; GIN indexed
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by trigger
    deleted_at                  TIMESTAMPTZ  NULL                              -- soft delete
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE catalog_items IS 'Service catalog. Binds a module version to a user-facing request form with cardinality (D25), layer identity (D24), tag defaults (L6) and visibility.';
COMMENT ON COLUMN catalog_items.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN catalog_items.module_version_id IS 'Pinned module version. FK module_versions(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN catalog_items.display_name IS 'User-facing name (e.g. "RDS MySQL"). Unique per module version among active items.';
COMMENT ON COLUMN catalog_items.description IS 'Human-readable description.';
COMMENT ON COLUMN catalog_items.category IS 'Grouping label (e.g. "database", "network").';
COMMENT ON COLUMN catalog_items.status IS 'Lifecycle status (proto CatalogItemStatus): draft|active|deprecated|archived|blocked.';
COMMENT ON COLUMN catalog_items.form_schema_json IS 'Frontend form schema (field definitions).';
COMMENT ON COLUMN catalog_items.defaults_json IS 'S2 catalog defaults (doc 08 param pipeline).';
COMMENT ON COLUMN catalog_items.cardinality IS 'D25: how codegen injects for_each/count: single|list|map.';
COMMENT ON COLUMN catalog_items.instance_key IS 'D25: variable name used as the for_each key.';
COMMENT ON COLUMN catalog_items.per_instance_fields_json IS 'D25: fields that vary per instance (read from each.value).';
COMMENT ON COLUMN catalog_items.shared_fields_json IS 'D25: fields shared across all instances.';
COMMENT ON COLUMN catalog_items.layer_logical_id IS 'Layer this catalog item belongs to. FK added by 010_layers (layer_logical_refs).';
COMMENT ON COLUMN catalog_items.stack_grouping IS 'D24 StackGranularity: per-component|per-bundle|per-team|custom.';
COMMENT ON COLUMN catalog_items.owner_team_id IS 'Team responsible for this catalog item. FK teams(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN catalog_items.default_tags_json IS 'L6 catalog default tags (doc 08 tag 7-layer model).';
COMMENT ON COLUMN catalog_items.user_allowed_tag_keys_json IS 'L7 user tag whitelist (doc 08; empty = no custom tags allowed).';
COMMENT ON COLUMN catalog_items.visibility_json IS 'team_ids that can see/request this item. GIN indexed.';
COMMENT ON COLUMN catalog_items.created_at IS 'Record creation time.';
COMMENT ON COLUMN catalog_items.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN catalog_items.deleted_at IS 'Soft delete timestamp. NULL = active.';

CREATE INDEX IF NOT EXISTS ix_catalog_items_module_version_id ON catalog_items(module_version_id);
CREATE INDEX IF NOT EXISTS ix_catalog_items_layer_logical_id ON catalog_items(layer_logical_id);
CREATE INDEX IF NOT EXISTS ix_catalog_items_owner_team_id ON catalog_items(owner_team_id);
-- GIN indexes for content-query JSONB fields (doc 08 visibility filter, tag whitelist).
CREATE INDEX IF NOT EXISTS ix_catalog_items_visibility ON catalog_items USING GIN(visibility_json);
CREATE INDEX IF NOT EXISTS ix_catalog_items_user_allowed_tag_keys ON catalog_items USING GIN(user_allowed_tag_keys_json);
CREATE UNIQUE INDEX IF NOT EXISTS uq_catalog_items_mv_display_active
    ON catalog_items(module_version_id, display_name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_catalog_items_updated_at
    BEFORE UPDATE ON catalog_items FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_catalog_items_updated_at ON catalog_items;
DROP INDEX IF EXISTS uq_catalog_items_mv_display_active;
DROP INDEX IF EXISTS ix_catalog_items_user_allowed_tag_keys;
DROP INDEX IF EXISTS ix_catalog_items_visibility;
DROP INDEX IF EXISTS ix_catalog_items_owner_team_id;
DROP INDEX IF EXISTS ix_catalog_items_layer_logical_id;
DROP INDEX IF EXISTS ix_catalog_items_module_version_id;
DROP TABLE IF EXISTS catalog_items;
