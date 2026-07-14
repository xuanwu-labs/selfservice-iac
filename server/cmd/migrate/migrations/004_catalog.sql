-- 004_catalog.sql: catalog_items — the service catalog (design.md §03 A3).
-- A catalog item binds a module version to a user-facing request form with
-- cardinality (D25), layer identity (D24), tag defaults (L6) and visibility.
-- layer_logical_id FK is back-filled by 010_layers (refs table created there).

-- +goose Up
CREATE TABLE IF NOT EXISTS catalog_items (
    id                          BIGINT       PRIMARY KEY,
    module_version_id           BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE RESTRICT,
    display_name                TEXT         NOT NULL,
    description                 TEXT         NOT NULL DEFAULT '',
    category                    TEXT         NOT NULL DEFAULT '',
    status                      TEXT         NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'active', 'deprecated', 'archived', 'blocked')),
    form_schema_json            JSONB        NOT NULL DEFAULT '{}',
    defaults_json               JSONB        NOT NULL DEFAULT '{}',           -- S2 catalog defaults (doc 08)
    cardinality                 TEXT         NOT NULL DEFAULT 'single'
                                CHECK (cardinality IN ('single', 'list', 'map')),  -- D25
    instance_key                TEXT         NOT NULL DEFAULT '',
    per_instance_fields_json    JSONB        NOT NULL DEFAULT '{}',
    shared_fields_json          JSONB        NOT NULL DEFAULT '{}',
    layer_logical_id            TEXT         NULL,                             -- FK added by 010_layers
    stack_grouping              TEXT         NOT NULL DEFAULT 'per-component'
                                CHECK (stack_grouping IN ('per-component', 'per-bundle', 'per-team', 'custom')),
    owner_team_id               BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    default_tags_json           JSONB        NOT NULL DEFAULT '{}',           -- L6 catalog defaults (doc 08)
    user_allowed_tag_keys_json  JSONB        NOT NULL DEFAULT '[]',           -- L7 user tag whitelist (doc 08)
    visibility_json             JSONB        NOT NULL DEFAULT '[]',           -- team_ids array
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                  TIMESTAMPTZ  NULL
);
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
