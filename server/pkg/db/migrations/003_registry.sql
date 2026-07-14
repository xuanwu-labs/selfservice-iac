-- 003_registry.sql: modules + module_versions + module_dependencies.
-- design.md §03 A2. Module registry: atomic TF modules registered with
-- version pins and pure-scalar variable contracts (D25 zero-intrusion).

-- +goose Up
CREATE TABLE IF NOT EXISTS modules (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    name            TEXT         NOT NULL,                              -- module name (e.g. "rds", "vpc", "ecs")
    git_source      TEXT         NOT NULL,                              -- git repo URL (e.g. github.com/org/modules)
    module_path     TEXT         NOT NULL DEFAULT '',                   -- subdir within repo (e.g. "atomic/rds")
    provider        TEXT         NOT NULL,                              -- cloud provider (e.g. "aliyun", "aws", "azure")
    layer           TEXT         NOT NULL DEFAULT '',                   -- informational layer; authoritative = catalog_items.layer_logical_id
    module_type     TEXT         NOT NULL DEFAULT 'atomic'
                    CHECK (module_type IN ('atomic', 'control', 'declarative')),  -- three-layer architecture
    owner_team_id   BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,  -- team responsible for this module
    status          TEXT         NOT NULL DEFAULT 'pending_validation'
                    CHECK (status IN ('pending_validation', 'validated', 'validation_failed', 'deprecated')),
    description     TEXT         NOT NULL DEFAULT '',                   -- human-readable description
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by trigger
    deleted_at      TIMESTAMPTZ  NULL                                   -- soft delete
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE modules IS 'Module registry entry. Atomic TF modules registered with version pins and pure-scalar variable contracts (D25 zero-intrusion).';
COMMENT ON COLUMN modules.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN modules.name IS 'Module name (e.g. "rds", "vpc", "ecs").';
COMMENT ON COLUMN modules.git_source IS 'Git repo URL (e.g. github.com/org/modules).';
COMMENT ON COLUMN modules.module_path IS 'Subdir within repo (e.g. "atomic/rds"). Empty = repo root.';
COMMENT ON COLUMN modules.provider IS 'Cloud provider (e.g. "aliyun", "aws", "azure").';
COMMENT ON COLUMN modules.layer IS 'Informational layer hint. Authoritative layer = catalog_items.layer_logical_id.';
COMMENT ON COLUMN modules.module_type IS 'Three-layer architecture type: atomic|control|declarative.';
COMMENT ON COLUMN modules.owner_team_id IS 'Team responsible for this module. FK teams(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN modules.status IS 'Validation lifecycle: pending_validation|validated|validation_failed|deprecated.';
COMMENT ON COLUMN modules.description IS 'Human-readable description.';
COMMENT ON COLUMN modules.created_at IS 'Record creation time.';
COMMENT ON COLUMN modules.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN modules.deleted_at IS 'Soft delete timestamp. NULL = active.';

CREATE INDEX IF NOT EXISTS ix_modules_owner_team_id ON modules(owner_team_id);
CREATE INDEX IF NOT EXISTS ix_modules_provider ON modules(provider);
CREATE INDEX IF NOT EXISTS ix_modules_status ON modules(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_modules_name_source_active
    ON modules(name, git_source) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_modules_updated_at
    BEFORE UPDATE ON modules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS module_versions (
    id                      BIGINT       PRIMARY KEY,
    module_id               BIGINT       NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    version                 TEXT         NOT NULL,                              -- semantic version (e.g. "v1.0.0")
    commit_sha              TEXT         NOT NULL,                              -- git commit hash (immutable pin)
    required_providers_json JSONB        NOT NULL DEFAULT '[]',                 -- from versions.tf: [{"name":"alicloud","version":">=1.280.0"}]
    variables_contract_json JSONB        NOT NULL DEFAULT '{}',                 -- from variables.tf: {name: {type,description,default,sensitive,required}}
    outputs_contract_json   JSONB        NOT NULL DEFAULT '{}',                 -- from outputs.tf: {name: {type,description}}
    is_current              BOOLEAN      NOT NULL DEFAULT FALSE,                -- marks the active version
    registered_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- when this version was registered
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE module_versions IS 'Pinned, immutable version of a module. Carries the pure-scalar variable/output contracts (D25).';
COMMENT ON COLUMN module_versions.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN module_versions.module_id IS 'Parent module. FK modules(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN module_versions.version IS 'Semantic version (e.g. "v1.0.0"). Unique per module.';
COMMENT ON COLUMN module_versions.commit_sha IS 'Git commit hash. Immutable pin the catalog item resolves against.';
COMMENT ON COLUMN module_versions.required_providers_json IS 'From versions.tf: [{"name":"alicloud","version":">=1.280.0"}].';
COMMENT ON COLUMN module_versions.variables_contract_json IS 'From variables.tf: {name: {type,description,default,sensitive,required}}.';
COMMENT ON COLUMN module_versions.outputs_contract_json IS 'From outputs.tf: {name: {type,description}}.';
COMMENT ON COLUMN module_versions.is_current IS 'Marks the active version of the module (at most one true per module_id).';
COMMENT ON COLUMN module_versions.registered_at IS 'When this version was registered.';
COMMENT ON COLUMN module_versions.created_at IS 'Record creation time.';

CREATE INDEX IF NOT EXISTS ix_module_versions_module_id ON module_versions(module_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_module_versions_module_version
    ON module_versions(module_id, version);

CREATE TABLE IF NOT EXISTS module_dependencies (
    id                  BIGINT       PRIMARY KEY,
    module_version_id   BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    variable_name       TEXT         NOT NULL,    -- this module's variable that needs upstream output (e.g. "vswitch_id")
    depends_on_layer    TEXT         NOT NULL,    -- upstream layer (e.g. "global")
    depends_on_module   TEXT         NOT NULL,    -- upstream module name (e.g. "vpc")
    output_key          TEXT         NOT NULL,    -- upstream module's output key (e.g. "vswitch_id"), validated against outputs_contract_json
    required            BOOLEAN      NOT NULL DEFAULT FALSE,  -- if true, codegen rejects when upstream not available
    description         TEXT         NOT NULL DEFAULT '',           -- human-readable dep description
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE module_dependencies IS 'Declares that a module version variable needs an upstream module output (D25 wiring).';
COMMENT ON COLUMN module_dependencies.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN module_dependencies.module_version_id IS 'Parent module version. FK module_versions(id) ON DELETE CASCADE.';
COMMENT ON COLUMN module_dependencies.variable_name IS 'This module''s variable that needs the upstream output (e.g. "vswitch_id").';
COMMENT ON COLUMN module_dependencies.depends_on_layer IS 'Upstream layer (e.g. "global").';
COMMENT ON COLUMN module_dependencies.depends_on_module IS 'Upstream module name (e.g. "vpc").';
COMMENT ON COLUMN module_dependencies.output_key IS 'Upstream module''s output key (e.g. "vswitch_id"), validated against outputs_contract_json.';
COMMENT ON COLUMN module_dependencies.required IS 'If true, codegen rejects when the upstream output is not available.';
COMMENT ON COLUMN module_dependencies.description IS 'Human-readable dependency description.';
COMMENT ON COLUMN module_dependencies.created_at IS 'Record creation time.';

CREATE INDEX IF NOT EXISTS ix_module_dependencies_module_version_id ON module_dependencies(module_version_id);

-- +goose Down
DROP INDEX IF EXISTS ix_module_dependencies_module_version_id;
DROP TABLE IF EXISTS module_dependencies;
DROP INDEX IF EXISTS uq_module_versions_module_version;
DROP INDEX IF EXISTS ix_module_versions_module_id;
DROP TABLE IF EXISTS module_versions;
DROP TRIGGER IF EXISTS trg_modules_updated_at ON modules;
DROP INDEX IF EXISTS uq_modules_name_source_active;
DROP INDEX IF EXISTS ix_modules_owner_team_id;
DROP TABLE IF EXISTS modules;
