-- 003_registry.sql: modules + module_versions + module_dependencies.
-- design.md §03 A2. Module registry: atomic TF modules registered with
-- version pins and pure-scalar variable contracts (D25 zero-intrusion).

-- +goose Up
CREATE TABLE IF NOT EXISTS modules (
    id              BIGINT       PRIMARY KEY,
    name            TEXT         NOT NULL,
    git_source      TEXT         NOT NULL,
    module_path     TEXT         NOT NULL DEFAULT '',  -- proto RegisterModuleRequest.module_path (subdir within git repo)
    provider        TEXT         NOT NULL,
    layer           TEXT         NOT NULL,
    owner_team_id   BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    status          TEXT         NOT NULL DEFAULT 'pending_validation'
                    CHECK (status IN ('pending_validation', 'validated', 'validation_failed', 'deprecated')),
    description     TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_modules_owner_team_id ON modules(owner_team_id);
CREATE INDEX IF NOT EXISTS ix_modules_provider ON modules(provider);
CREATE INDEX IF NOT EXISTS ix_modules_status ON modules(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_modules_name_source
    ON modules(name, git_source);
CREATE TRIGGER trg_modules_updated_at
    BEFORE UPDATE ON modules FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS module_versions (
    id                      BIGINT       PRIMARY KEY,
    module_id               BIGINT       NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    version                 TEXT         NOT NULL,
    commit_sha              TEXT         NOT NULL,
    providers_json          JSONB        NOT NULL DEFAULT '{}',
    variables_contract_json JSONB        NOT NULL DEFAULT '{}',  -- pure scalar contract (D25), S1 pipeline input
    is_current              BOOLEAN      NOT NULL DEFAULT FALSE,
    registered_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_module_versions_module_id ON module_versions(module_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_module_versions_module_version
    ON module_versions(module_id, version);

CREATE TABLE IF NOT EXISTS module_dependencies (
    id                  BIGINT       PRIMARY KEY,
    module_version_id   BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    variable_name       TEXT         NOT NULL,
    depends_on_layer    TEXT         NOT NULL,
    depends_on_module   TEXT         NOT NULL,
    output_key          TEXT         NOT NULL,
    required            BOOLEAN      NOT NULL DEFAULT FALSE,
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
DROP INDEX IF EXISTS uq_modules_name_source;
DROP INDEX IF EXISTS ix_modules_owner_team_id;
DROP TABLE IF EXISTS modules;
