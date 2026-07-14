-- 008_cloud.sql: cloud_accounts (design.md §03 A6).
-- Aligned to proto CloudAccountStatus (active/suspended). The doc 07c §14
-- cascade (deprecating/deprecated) is an ops lifecycle not in proto —
-- deferred to a future proto change when cascade is implemented.

-- +goose Up
CREATE TABLE IF NOT EXISTS cloud_accounts (
    id                      BIGINT       PRIMARY KEY,                       -- snowflake ID
    provider                TEXT         NOT NULL                           -- proto CloudProvider
                            CHECK (provider IN ('aws', 'aliyun', 'azure', 'gcp')),  -- proto CloudProvider
    account_id              TEXT         NOT NULL,                          -- cloud-native account number/id (unique per provider)
    name                    TEXT         NOT NULL DEFAULT '',  -- proto CloudAccount.name
    alias                   TEXT         NOT NULL DEFAULT '',               -- short human alias for the account
    display_name            TEXT         NOT NULL DEFAULT '',               -- friendly name shown in UI
    status                  TEXT         NOT NULL DEFAULT 'active'          -- account lifecycle (proto CloudAccountStatus)
                            CHECK (status IN ('active', 'suspended')),  -- proto CloudAccountStatus
    default_region          TEXT         NOT NULL DEFAULT '',               -- primary region when none is specified
    regions_json            JSONB        NOT NULL DEFAULT '[]',             -- enabled regions: ["us-east-1", ...]
    credentials_ref         TEXT         NOT NULL DEFAULT '',   -- Vault/KMS ref, never the secret itself
    billing_enabled         BOOLEAN      NOT NULL DEFAULT FALSE,            -- whether cost/billing export is enabled
    default_team_id         BIGINT       NULL REFERENCES teams(id) ON DELETE SET NULL,  -- FK teams - default owning team
    tags_json               JSONB        NOT NULL DEFAULT '{}',             -- arbitrary tags: {key: value}
    bootstrap_status        TEXT         NOT NULL DEFAULT 'none'            -- credential bootstrap state (none/ok/rotate)
                            CHECK (bootstrap_status IN ('ok', 'rotate', 'none')),
    oidc_trust_configured   BOOLEAN      NOT NULL DEFAULT FALSE,            -- whether workload-identity OIDC trust is set up
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),            -- row creation time
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()             -- last update time (trigger-maintained)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE cloud_accounts IS 'Cloud account registration (design.md §03 A6). Aligned to proto CloudAccountStatus (active/suspended).';
COMMENT ON COLUMN cloud_accounts.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN cloud_accounts.provider IS 'proto CloudProvider: aws|aliyun|azure|gcp.';
COMMENT ON COLUMN cloud_accounts.account_id IS 'Cloud-native account number/id. Unique per provider.';
COMMENT ON COLUMN cloud_accounts.name IS 'proto CloudAccount.name.';
COMMENT ON COLUMN cloud_accounts.alias IS 'Short human alias for the account.';
COMMENT ON COLUMN cloud_accounts.display_name IS 'Friendly name shown in UI.';
COMMENT ON COLUMN cloud_accounts.status IS 'Account lifecycle (proto CloudAccountStatus): active|suspended.';
COMMENT ON COLUMN cloud_accounts.default_region IS 'Primary region used when none is specified.';
COMMENT ON COLUMN cloud_accounts.regions_json IS 'Enabled regions: ["us-east-1", ...].';
COMMENT ON COLUMN cloud_accounts.credentials_ref IS 'Vault/KMS ref, never the secret itself.';
COMMENT ON COLUMN cloud_accounts.billing_enabled IS 'Whether cost/billing export is enabled.';
COMMENT ON COLUMN cloud_accounts.default_team_id IS 'Default owning team. FK teams(id) ON DELETE SET NULL.';
COMMENT ON COLUMN cloud_accounts.tags_json IS 'Arbitrary tags: {key: value}.';
COMMENT ON COLUMN cloud_accounts.bootstrap_status IS 'Credential bootstrap state: ok|rotate|none.';
COMMENT ON COLUMN cloud_accounts.oidc_trust_configured IS 'Whether workload-identity OIDC trust is set up.';
COMMENT ON COLUMN cloud_accounts.created_at IS 'Row creation time.';
COMMENT ON COLUMN cloud_accounts.updated_at IS 'Last update time (trigger-maintained).';

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
