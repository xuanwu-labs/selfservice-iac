-- 002_organizations.sql: projects + spaces (teams is in 001).
-- design.md §03 A1. projects belong to a team; spaces group stacks within
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
COMMENT ON TABLE projects IS 'Project entity owned by a team. Groups spaces and carries the team ownership root.';
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

-- layer_logical_refs must exist before spaces (FK). Created in 010_layers,
-- but spaces references it — so we create the minimal refs table here and
-- 010_layers seeds it. To keep migrations ordered without circular deps,
-- layer_logical_refs + layer_rule_set_versions live in 010 and spaces
-- adds the FK via ALTER in 010. For now spaces is created without the FK
-- and 010 back-fills it.
CREATE TABLE IF NOT EXISTS spaces (
    id          BIGINT       PRIMARY KEY,                    -- snowflake ID
    name        TEXT         NOT NULL,                        -- space display name (e.g. "orders")
    project_id  BIGINT       NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,  -- parent project
    layer_logical_id TEXT    NULL,  -- FK added by 010_layers; layer this space belongs to
    repo_path   TEXT         NOT NULL,                        -- repo path for this space's stacks
    tags_json   JSONB        NOT NULL DEFAULT '{}',           -- L4 space tags (doc 08 tag 7-layer model)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by trigger
    deleted_at  TIMESTAMPTZ  NULL                             -- soft delete
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE spaces IS 'Space of stacks within a project. Carries the layer identity used for path generation (D24).';
COMMENT ON COLUMN spaces.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN spaces.name IS 'Space display name (e.g. "orders"). Unique per project among active (non-deleted) spaces.';
COMMENT ON COLUMN spaces.project_id IS 'Parent project. FK projects(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN spaces.layer_logical_id IS 'Layer this space belongs to. FK added by 010_layers (layer_logical_refs).';
COMMENT ON COLUMN spaces.repo_path IS 'Repo path for this space''s stacks. Used by PathGenerator.';
COMMENT ON COLUMN spaces.tags_json IS 'L4 space tags (doc 08 tag 7-layer model).';
COMMENT ON COLUMN spaces.created_at IS 'Record creation time.';
COMMENT ON COLUMN spaces.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN spaces.deleted_at IS 'Soft delete timestamp. NULL = active.';

CREATE INDEX IF NOT EXISTS ix_spaces_project_id ON spaces(project_id);
CREATE INDEX IF NOT EXISTS ix_spaces_layer_logical_id ON spaces(layer_logical_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_spaces_project_name_active
    ON spaces(project_id, name) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_spaces_updated_at
    BEFORE UPDATE ON spaces FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_spaces_updated_at ON spaces;
DROP INDEX IF EXISTS uq_spaces_project_name_active;
DROP INDEX IF EXISTS ix_spaces_layer_logical_id;
DROP INDEX IF EXISTS ix_spaces_project_id;
DROP TABLE IF EXISTS spaces;
DROP TRIGGER IF EXISTS trg_projects_updated_at ON projects;
DROP INDEX IF EXISTS uq_projects_team_name_active;
DROP INDEX IF EXISTS ix_projects_team_id;
DROP TABLE IF EXISTS projects;
