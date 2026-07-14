-- 002_organizations.sql: projects + bundles (teams is in 001).
-- design.md §03 A1. projects belong to a team; bundles group stacks within
-- a project and carry the layer identity for path generation (D24).

-- +goose Up
CREATE TABLE IF NOT EXISTS projects (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    team_id     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ  NULL
);
CREATE INDEX IF NOT EXISTS ix_projects_team_id ON projects(team_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_projects_team_name_active
    ON projects(team_id, name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_projects_updated_at
    BEFORE UPDATE ON projects FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- layer_logical_refs must exist before bundles (FK). Created in 010_layers,
-- but bundles references it — so we create the minimal refs table here and
-- 010_layers seeds it. To keep migrations ordered without circular deps,
-- layer_logical_refs + layer_rule_set_versions live in 010 and bundles
-- adds the FK via ALTER in 010. For now bundles is created without the FK
-- and 010 back-fills it.
CREATE TABLE IF NOT EXISTS bundles (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    project_id  BIGINT       NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    layer_logical_id TEXT    NULL,  -- FK added by 010_layers after layer_logical_refs exists
    repo_path   TEXT         NOT NULL,
    tags_json   JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ  NULL
);
CREATE INDEX IF NOT EXISTS ix_bundles_project_id ON bundles(project_id);
CREATE INDEX IF NOT EXISTS ix_bundles_layer_logical_id ON bundles(layer_logical_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_bundles_project_name_active
    ON bundles(project_id, name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_bundles_updated_at
    BEFORE UPDATE ON bundles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_bundles_updated_at ON bundles;
DROP INDEX IF EXISTS uq_bundles_project_name_active;
DROP INDEX IF EXISTS ix_bundles_layer_logical_id;
DROP INDEX IF EXISTS ix_bundles_project_id;
DROP TABLE IF EXISTS bundles;
DROP TRIGGER IF EXISTS trg_projects_updated_at ON projects;
DROP INDEX IF EXISTS uq_projects_team_name_active;
DROP INDEX IF EXISTS ix_projects_team_id;
DROP TABLE IF EXISTS projects;
