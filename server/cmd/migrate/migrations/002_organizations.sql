-- 002_organizations.sql: projects + bundles (teams is in 001).
-- design.md §03 A1. projects belong to a team; bundles group stacks within
-- a project and carry the layer identity for path generation (D24).

-- +goose Up
CREATE TABLE IF NOT EXISTS projects (
    id          BIGINT       PRIMARY KEY,                    -- snowflake ID
    name        TEXT         NOT NULL,                        -- project display name
    team_id     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,  -- owning team
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by trigger
    deleted_at  TIMESTAMPTZ  NULL                             -- soft delete
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE projects IS 'Project entity owned by a team. Groups bundles and carries the team ownership root.';
COMMENT ON COLUMN projects.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN projects.name IS 'Project display name. Unique per team among active (non-deleted) projects.';
COMMENT ON COLUMN projects.team_id IS 'Owning team. FK teams(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN projects.created_at IS 'Record creation time.';
COMMENT ON COLUMN projects.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN projects.deleted_at IS 'Soft delete timestamp. NULL = active.';

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
    id          BIGINT       PRIMARY KEY,                    -- snowflake ID
    name        TEXT         NOT NULL,                        -- bundle display name (e.g. "orders")
    project_id  BIGINT       NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,  -- parent project
    layer_logical_id TEXT    NULL,  -- FK added by 010_layers; layer this bundle belongs to
    repo_path   TEXT         NOT NULL,                        -- repo path for this bundle's stacks
    tags_json   JSONB        NOT NULL DEFAULT '{}',           -- L4 bundle tags (doc 08 tag 7-layer model)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by trigger
    deleted_at  TIMESTAMPTZ  NULL                             -- soft delete
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE bundles IS 'Bundle of stacks within a project. Carries the layer identity used for path generation (D24).';
COMMENT ON COLUMN bundles.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN bundles.name IS 'Bundle display name (e.g. "orders"). Unique per project among active (non-deleted) bundles.';
COMMENT ON COLUMN bundles.project_id IS 'Parent project. FK projects(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN bundles.layer_logical_id IS 'Layer this bundle belongs to. FK added by 010_layers (layer_logical_refs).';
COMMENT ON COLUMN bundles.repo_path IS 'Repo path for this bundle''s stacks. Used by PathGenerator.';
COMMENT ON COLUMN bundles.tags_json IS 'L4 bundle tags (doc 08 tag 7-layer model).';
COMMENT ON COLUMN bundles.created_at IS 'Record creation time.';
COMMENT ON COLUMN bundles.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN bundles.deleted_at IS 'Soft delete timestamp. NULL = active.';

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
