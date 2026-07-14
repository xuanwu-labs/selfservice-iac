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
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ  NULL                                   -- soft delete
);
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
    registered_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);
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
    description         TEXT         NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);
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
