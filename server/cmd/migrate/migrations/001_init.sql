-- +goose Up
-- teams table (aligned with iac-self-service-platform/docs/04)
CREATE TABLE IF NOT EXISTS teams (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(128) NOT NULL,
    slug       VARCHAR(64)  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT teams_slug_uk UNIQUE (slug)
);

CREATE INDEX IF NOT EXISTS teams_name_idx ON teams (name);

-- +goose Down
DROP INDEX IF EXISTS teams_name_idx;
DROP TABLE IF EXISTS teams;
