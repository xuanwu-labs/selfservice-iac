-- 013_env_tenant_tagpolicy.sql: environments + tenants + environment_tenant_bindings + tag_policies
--
-- Closes the W1 prerequisite gap: doc 04 §2.13/§2.14 define these tables as
-- first-class governance objects (D27 env/tenant, D28 tag namespace). Without
-- them, codegen Stage 4 (EnvironmentTenantBinding resolver) has no backing store.
--
-- Schema authority: docs/04 §2.13 (env/tenant/binding) + §2.14 (tag_policies) + docs/07.
--
-- Note (D27 scope): stacks.env (TEXT) and stacks.tenant_id (TEXT) remain dangling
-- string slugs per migration 011 — they are NOT migrated to BIGINT FKs to
-- environments.id/tenants.id in this migration. The parallel BIGINT-keyed world
-- introduced here is consumed by environment_tenant_bindings and future codegen;
-- the TEXT->BIGINT migration of stacks.env/requests.env_id is a tracked follow-up
-- (W1-04 stackmodel, when codegen Stage 4 resolves bindings). Identity/
-- orchestration/drift/approval tables remain deferred to W2 modules (YAGNI).
--
-- Schema authority: docs/04 §2.13 (env/tenant/binding) + §2.14 (tag_policies) + docs/07.

-- +goose Up

-- =====================================================================
-- 1. environments: Environment as a first-class governance object (D27).
--    stage drives governance strength (prod=strong approval, dev=auto-admit).
--    env_logical_id is the stable identity (dev/staging/prod/dr); id is snowflake.
--    Authority: docs/04 §2.13.
-- =====================================================================
CREATE TABLE IF NOT EXISTS environments (
    id                      BIGINT       PRIMARY KEY,
    env_logical_id          TEXT         NOT NULL,                                -- stable identity: dev/staging/prod/dr
    display_name            TEXT         NOT NULL DEFAULT '',                      -- human-friendly name
    stage                   TEXT         NOT NULL DEFAULT 'dev'
                            CHECK (stage IN ('dev', 'staging', 'prod', 'dr')),   -- governance strength driver
    cloud_account_id        BIGINT       NULL REFERENCES cloud_accounts(id) ON DELETE SET NULL,  -- default cloud account (binding can override)
    region                  TEXT         NOT NULL DEFAULT '',                      -- primary region (e.g. cn-hangzhou)
    network_topology        TEXT         NOT NULL DEFAULT 'shared'
                            CHECK (network_topology IN ('shared', 'distributed')),
    tag_namespace_json      JSONB        NOT NULL DEFAULT '{}',                    -- D28 L2 tag source: env-level tag defaults
    status                  TEXT         NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'frozen', 'deprecated')),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ  NULL
);

COMMENT ON TABLE environments IS 'Environment as first-class governance object (D27). stage drives approval strength; env_logical_id is the stable identity.';
COMMENT ON COLUMN environments.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN environments.env_logical_id IS 'Stable identity: dev/staging/prod/dr. Distinct from display_name.';
COMMENT ON COLUMN environments.stage IS 'Governance strength driver: prod=strong approval, dev=auto-admit.';
COMMENT ON COLUMN environments.cloud_account_id IS 'Default cloud account; environment_tenant_bindings.override_cloud_account_id takes precedence for account-per-env.';
COMMENT ON COLUMN environments.tag_namespace_json IS 'D28 L2 tag source: env-level tag defaults merged in codegen Stage 8.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_environments_env_logical_id_active ON environments(env_logical_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_environments_stage ON environments(stage) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_environments_cloud_account_id ON environments(cloud_account_id);

CREATE TRIGGER trg_environments_updated_at
    BEFORE UPDATE ON environments FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 2. tenants: Tenant as first-class object (D27). Default 'platform-default'
--    means "internal shared". External customers / separate BUs get own tenant.
--    isolation_level is 2-tier (industry converged 2026+).
--    Authority: docs/04 §2.13.
-- =====================================================================
CREATE TABLE IF NOT EXISTS tenants (
    id                      BIGINT       PRIMARY KEY,
    tenant_logical_id       TEXT         NOT NULL,                                -- stable identity (e.g. platform-default, corp-a)
    name                    TEXT         NOT NULL,                                -- display name
    isolation_level         TEXT         NOT NULL DEFAULT 'vpc-per-env'
                            CHECK (isolation_level IN ('vpc-per-env', 'account-per-env')),  -- 2-tier only (D27 simplification)
    kind                    TEXT         NOT NULL DEFAULT 'internal'
                            CHECK (kind IN ('internal', 'external')),            -- external customer / separate BU; informational, does not drive isolation
    owner_team_id           BIGINT       NULL REFERENCES teams(id) ON DELETE SET NULL,  -- governance owner team
    tag_namespace_json      JSONB        NOT NULL DEFAULT '{}',                    -- D28 L3 tag source: tenant-level tag
    status                  TEXT         NOT NULL DEFAULT 'active'
                            CHECK (status IN ('active', 'frozen', 'deprecated')),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at              TIMESTAMPTZ  NULL
);

COMMENT ON TABLE tenants IS 'Tenant as first-class object (D27). Default platform-default = internal shared. isolation_level is 2-tier only.';
COMMENT ON COLUMN tenants.tenant_logical_id IS 'Stable identity. platform-default is seeded as the internal shared tenant.';
COMMENT ON COLUMN tenants.isolation_level IS 'vpc-per-env (default) = one tenant x one env = one VPC. account-per-env = escape hatch, each tenant+env independent cloud account.';
COMMENT ON COLUMN tenants.tag_namespace_json IS 'D28 L3 tag source: tenant-level tag, merged in codegen Stage 8.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenants_tenant_logical_id_active ON tenants(tenant_logical_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_tenants_owner_team_id ON tenants(owner_team_id);

CREATE TRIGGER trg_tenants_updated_at
    BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 3. environment_tenant_bindings: (env x tenant x layer) triple UNIQUE.
--    Network scope resolver for codegen Stage 4. Application-layer stack
--    creation requires a matching binding or codegen rejects.
--    Authority: docs/04 §2.13.
-- =====================================================================
CREATE TABLE IF NOT EXISTS environment_tenant_bindings (
    id                          BIGINT       PRIMARY KEY,
    env_id                      BIGINT       NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    tenant_id                   BIGINT       NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    layer_logical_id            TEXT         NOT NULL REFERENCES layer_logical_refs(logical_id) ON DELETE RESTRICT,  -- triple: (env, tenant, layer)
    vpc_stack_id                BIGINT       NULL REFERENCES stacks(id) ON DELETE SET NULL,  -- Global-layer VPC stack (outputs vpc_id/vswitch_ids); nullable for Global layer itself
    subnet_blocks_json          JSONB        NOT NULL DEFAULT '[]',                    -- subnet CIDR list
    security_group_base_id      TEXT         NOT NULL DEFAULT '',                      -- base security group id for this tenant-env
    override_cloud_account_id   BIGINT       NULL REFERENCES cloud_accounts(id) ON DELETE SET NULL,  -- account-per-env required; overrides environments.cloud_account_id
    status                      TEXT         NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'pending-cleanup')),
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE environment_tenant_bindings IS '(env x tenant x layer) triple = network scope resolver (D27). codegen Stage 4 query entry.';
COMMENT ON COLUMN environment_tenant_bindings.layer_logical_id IS 'FK layer_logical_refs. Triple (env_id, tenant_id, layer_logical_id) is UNIQUE.';
COMMENT ON COLUMN environment_tenant_bindings.vpc_stack_id IS 'MUST reference Global-layer stack with vpc_id+vswitch_ids outputs. Nullable for Global layer binding itself.';
COMMENT ON COLUMN environment_tenant_bindings.override_cloud_account_id IS 'account-per-env required (MUST differ from environments.cloud_account_id). NULL = use env default.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_env_tenant_layer ON environment_tenant_bindings(env_id, tenant_id, layer_logical_id);
CREATE INDEX IF NOT EXISTS ix_env_tenant_bindings_env ON environment_tenant_bindings(env_id);
CREATE INDEX IF NOT EXISTS ix_env_tenant_bindings_tenant ON environment_tenant_bindings(tenant_id);

CREATE TRIGGER trg_env_tenant_bindings_updated_at
    BEFORE UPDATE ON environment_tenant_bindings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 4. tag_policies: D28 tag namespace config storage (L2/L3/L4/L6 layers).
--    scope_type polymorphic reference (scope_id is TEXT, not FK — integrity
--    enforced at app layer). Per docs/04 §2.14:
--      - platform scope stores L1-mandated tag namespace config (the table IS
--        the L1 source of truth; "derived" L1 refers to per-stack tag VALUES
--        computed by codegen, not the policy config itself).
--      - env/tenant/team/space/catalog_item store L2/L3/L4/L6 layer config.
--      - L5 (per-stack) tag VALUES are derived by codegen from stacks metadata,
--        not stored as a policy row.
-- =====================================================================
CREATE TABLE IF NOT EXISTS tag_policies (
    id                              BIGINT       PRIMARY KEY,
    scope_type                      TEXT         NOT NULL
                                    CHECK (scope_type IN ('platform', 'env', 'tenant', 'team', 'space', 'catalog_item')),
    scope_id                        TEXT         NOT NULL,                        -- polymorphic: platform='platform', else entity id as text
    tag_namespace_json              JSONB        NOT NULL DEFAULT '{}',           -- forced tag key-value pairs
    mandatory_keys_json             JSONB        NOT NULL DEFAULT '[]',           -- keys that MUST exist (value can be empty)
    user_allowed_tag_keys_json      JSONB        NOT NULL DEFAULT '[]',           -- user whitelist (empty = no custom tags allowed)
    version                         INT          NOT NULL DEFAULT 1,              -- monotonic; old versions kept for audit
    created_at                      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                      TIMESTAMPTZ  NULL
);

COMMENT ON TABLE tag_policies IS 'D28 tag namespace config (L2/L3/L4/L6 layers). Polymorphic scope; integrity at app layer.';
COMMENT ON COLUMN tag_policies.scope_type IS 'Layer source: platform/env/tenant/team/space/catalog_item. L1/L5 derived by codegen, not stored here.';
COMMENT ON COLUMN tag_policies.scope_id IS 'Polymorphic: scope_type=platform => literal "platform"; else entity id as text. Integrity enforced at app layer.';
COMMENT ON COLUMN tag_policies.mandatory_keys_json IS 'Keys that MUST exist (value can be empty). codegen Stage 9 rejects if missing.';
COMMENT ON COLUMN tag_policies.user_allowed_tag_keys_json IS 'User whitelist for custom tags. Empty = user cannot add any custom tag.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_tag_policies_scope_active ON tag_policies(scope_type, scope_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_tag_policies_scope_type ON tag_policies(scope_type) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_tag_policies_updated_at
    BEFORE UPDATE ON tag_policies FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 5. Seeds: default tenant + default envs + platform-level tag policy.
--    Note: layer_rule_set_versions v1 already seeded by migration 010.
-- =====================================================================

-- Default tenant (internal shared semantics)
INSERT INTO tenants (id, tenant_logical_id, name, isolation_level, kind, tag_namespace_json, status, created_at)
VALUES (0, 'platform-default', 'Platform Default (Internal Shared)', 'vpc-per-env', 'internal', '{}'::jsonb, 'active', now())
ON CONFLICT (tenant_logical_id) DO NOTHING;

-- Default environments (stable identities). IDs 1-4 reserved for seeds.
INSERT INTO environments (id, env_logical_id, display_name, stage, region, tag_namespace_json, status, created_at) VALUES
    (1, 'dev',     'Development',     'dev',     '', '{}'::jsonb, 'active', now()),
    (2, 'staging', 'Staging',         'staging', '', '{}'::jsonb, 'active', now()),
    (3, 'prod',    'Production',      'prod',    '', '{}'::jsonb, 'active', now()),
    (4, 'dr',      'Disaster Recovery','dr',     '', '{}'::jsonb, 'active', now())
ON CONFLICT (env_logical_id) DO NOTHING;

-- Platform-level mandatory tag policy (D28 L1 baseline)
INSERT INTO tag_policies (id, scope_type, scope_id, tag_namespace_json, mandatory_keys_json, user_allowed_tag_keys_json, version, created_at)
VALUES (
    0,
    'platform',
    'platform',
    '{}'::jsonb,
    '["platform-managed","platform-team","platform-space","platform-stack"]'::jsonb,
    '[]'::jsonb,
    1,
    now()
)
ON CONFLICT (scope_type, scope_id) DO NOTHING;


-- +goose Down

-- Drop triggers BEFORE tables (Postgres DROP TABLE CASCADE auto-drops triggers,
-- but DROP TRIGGER ON <gone-table> errors with "relation does not exist").
DROP TRIGGER IF EXISTS trg_tag_policies_updated_at ON tag_policies;
DROP TRIGGER IF EXISTS trg_env_tenant_bindings_updated_at ON environment_tenant_bindings;
DROP TRIGGER IF EXISTS trg_tenants_updated_at ON tenants;
DROP TRIGGER IF EXISTS trg_environments_updated_at ON environments;

DROP TABLE IF EXISTS tag_policies;
DROP TABLE IF EXISTS environment_tenant_bindings;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS environments;
