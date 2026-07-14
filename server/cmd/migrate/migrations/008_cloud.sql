-- 008_cloud.sql: cloud_accounts (design.md §03 A6).
-- Aligned to doc 04 §2.12 (authoritative; doc 06 footnotes defer). Status
-- includes deprecating/deprecated to support the doc 07c §14 cascade.

-- +goose Up
CREATE TABLE IF NOT EXISTS cloud_accounts (
    id                      BIGINT       PRIMARY KEY,
    provider                TEXT         NOT NULL
                            CHECK (provider IN ('aws', 'aliyun', 'azure', 'gcp')),  -- proto CloudProvider
    account_id              TEXT         NOT NULL,
    name                    TEXT         NOT NULL DEFAULT '',  -- proto CloudAccount.name
    alias                   TEXT         NOT NULL DEFAULT '',
    display_name            TEXT         NOT NULL DEFAULT '',
    status                  TEXT         NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'suspended')),  -- proto CloudAccountStatus
    default_region          TEXT         NOT NULL DEFAULT '',
    regions_json            JSONB        NOT NULL DEFAULT '[]',
    credentials_ref         TEXT         NOT NULL DEFAULT '',   -- Vault/KMS ref, never the secret itself
    billing_enabled         BOOLEAN      NOT NULL DEFAULT FALSE,
    default_team_id         BIGINT       NULL REFERENCES teams(id) ON DELETE SET NULL,
    tags_json               JSONB        NOT NULL DEFAULT '{}',
    bootstrap_status        TEXT         NOT NULL DEFAULT 'none'
                            CHECK (bootstrap_status IN ('ok', 'rotate', 'none')),
    oidc_trust_configured   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_cloud_accounts_default_team_id ON cloud_accounts(default_team_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_cloud_accounts_provider_account
    ON cloud_accounts(provider, account_id);
CREATE TRIGGER trg_cloud_accounts_updated_at
    BEFORE UPDATE ON cloud_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_cloud_accounts_updated_at ON cloud_accounts;
DROP INDEX IF EXISTS uq_cloud_accounts_provider_account;
DROP INDEX IF EXISTS ix_cloud_accounts_default_team_id;
DROP TABLE IF EXISTS cloud_accounts;
