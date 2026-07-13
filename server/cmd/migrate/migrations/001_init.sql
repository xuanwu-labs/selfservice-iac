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
CREATE TABLE IF NOT EXISTS teams (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    slug        TEXT         NOT NULL,
    kind        TEXT         NOT NULL CHECK (kind IN ('platform', 'dba', 'middleware', 'business')),
    status      TEXT         NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated')),
    tags_json   JSONB        NOT NULL DEFAULT '{}',
    policy_json JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ  NULL,
    CONSTRAINT pk_teams PRIMARY KEY (id)
);

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
