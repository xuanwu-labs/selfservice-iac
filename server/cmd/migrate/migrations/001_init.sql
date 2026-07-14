-- 001_init.sql: teams table — the organizational ownership root.
--
-- All other MVP tables reference teams(id). Aligned to design.md §03 A1
-- (platform-db-schema): snowflake BIGINT PK (app-generated, no DB autoincrement),
-- kind discriminator, status lifecycle, JSONB policy/tags, soft-delete via
-- deleted_at, updated_at trigger, snake_case + pk_/uq_/ix_ naming.
--
-- Skill compliance (postgresql-table-design):
--   - TEXT (not VARCHAR(n))
--   - TIMESTAMPTZ (not TIMESTAMP / timestamptz(n))
--   - BIGINT PK (snowflake, no serial)
--   - FK columns indexed (teams has none — it's the referenced parent)
--   - Soft-delete unique via partial index WHERE deleted_at IS NULL

-- +goose Up
-- Shared trigger function for updated_at auto-maintenance (was 000_utils.sql;
-- goose skips version 0, so merged here). Every business table with updated_at
-- attaches this via:
--   CREATE TRIGGER trg_<table>_updated_at BEFORE UPDATE ON <table>
--     FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- See design.md §2.2. sqlc does not manage audit columns.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS teams (
    id          BIGINT       PRIMARY KEY,                    -- snowflake ID
    name        TEXT         NOT NULL,                        -- team display name (e.g. "DBA Team")
    slug        TEXT         NOT NULL,                        -- URL-safe identifier (e.g. "dba")
    kind        TEXT         NOT NULL CHECK (kind IN ('platform', 'dba', 'middleware', 'business')),  -- team type, matched by layer owning_team_pattern
    status      TEXT         NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated')),  -- lifecycle status
    tags_json   JSONB        NOT NULL DEFAULT '{}',           -- L4 team tags (doc 08 tag 7-layer model)
    policy_json JSONB        NOT NULL DEFAULT '{}',           -- S6 team policy (allowed_regions, cost_cap, mandatory_tags)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- auto-maintained by set_updated_at() trigger
    deleted_at  TIMESTAMPTZ  NULL                             -- soft delete timestamp
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE teams IS 'Organizational ownership root. All other MVP tables reference teams(id).';
COMMENT ON COLUMN teams.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN teams.name IS 'Team display name (e.g. "DBA Team").';
COMMENT ON COLUMN teams.slug IS 'URL-safe identifier (e.g. "dba"). Unique among active (non-deleted) teams.';
COMMENT ON COLUMN teams.kind IS 'Team type: platform|dba|middleware|business. Matched by layer_rule_set_versions.layers_json.owning_team_pattern.';
COMMENT ON COLUMN teams.status IS 'Lifecycle status: active|deprecated.';
COMMENT ON COLUMN teams.tags_json IS 'L4 team tags (doc 08 tag 7-layer model).';
COMMENT ON COLUMN teams.policy_json IS 'S6 team policy: {allowed_regions, cost_cap, mandatory_tags} (doc 08 param pipeline).';
COMMENT ON COLUMN teams.created_at IS 'Record creation time.';
COMMENT ON COLUMN teams.updated_at IS 'Auto-maintained by set_updated_at() trigger.';
COMMENT ON COLUMN teams.deleted_at IS 'Soft delete timestamp. NULL = active.';

-- Soft-delete-aware uniqueness: only one active team per slug; deleted rows
-- can be re-created with the same slug. Uses a partial unique index per
-- design.md §2.1 (NULLS NOT DISTINCT alternative for soft-delete scenarios).
CREATE UNIQUE INDEX IF NOT EXISTS uq_teams_slug_active
    ON teams (slug)
    WHERE deleted_at IS NULL;

-- Lookup index by kind (team-type filters, e.g. "list all DBA teams").
CREATE INDEX IF NOT EXISTS ix_teams_kind ON teams (kind);

-- Attach the shared updated_at trigger (000_utils.sql).
CREATE TRIGGER trg_teams_updated_at
    BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_teams_updated_at ON teams;
DROP INDEX IF EXISTS ix_teams_kind;
DROP INDEX IF EXISTS uq_teams_slug_active;
DROP TABLE IF EXISTS teams;
DROP FUNCTION IF EXISTS set_updated_at();
