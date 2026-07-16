-- 011_exec_stack_state.sql: state_backends + stacks + stack_dependencies
-- + workspaces + workspace_checkouts.
--
-- Closes the end-to-end gap identified in the 2026-07-16 architecture review:
-- the MVP 20-table set only carried the catalog -> request -> codegen-input
-- half. Execution-plane entities (stack metadata, git workspace, state backend
-- config) were design-only (doc 04 sections 2.3 / 2.5 / 2.10) with no
-- migration. Without them pinned_commit in requests is an orphan, codegen has
-- nowhere to persist generated stack identity, and backend.tf had no DB source
-- of truth (doc 02 hard-coded bucket = "tm-state").
--
-- This migration promotes the five execution-plane tables to MVP:
--   * state_backends        - backend config source of truth (L1 platform-wide)
--   * stacks                - generated stack identity (D29 stack.tm.hcl mirror)
--   * stack_dependencies    - runtime cross-layer wiring (D29 after/watch)
--   * workspaces            - the infra-repo registration (D4)
--   * workspace_checkouts   - per-request worktree + pinned_commit (D4/D21)
--
-- It also drops modules.module_type: registry only accepts atomic modules
-- (D19/D25); control-layer orchestration is the platform codegen's job and
-- declarative input is the web form's job, so the three-value CHECK was a
-- carry-over from the human-Terraform layering in terraform-alicloud-modules.

-- +goose Up

-- =====================================================================
-- 1. Remove modules.module_type (registry is atomic-only, D19/D25).
-- =====================================================================
DROP INDEX IF EXISTS ix_modules_module_type;
ALTER TABLE modules DROP COLUMN IF EXISTS module_type;

-- =====================================================================
-- 2. state_backends: S3/OSS bucket config (L1 platform-wide, single default).
--    Replaces the hard-coded bucket = "tm-state" in doc 02 section 4.1 and
--    doc 09 section 6. codegen reads this row to render backend.tf; the
--    per-stack key still comes from PathGenerator (D24/D29).
-- =====================================================================
CREATE TABLE IF NOT EXISTS state_backends (
    id              BIGINT       PRIMARY KEY,                     -- snowflake ID
    name            TEXT         NOT NULL,                         -- stable name, e.g. "default-s3"
    kind            TEXT         NOT NULL                          -- backend type
                    CHECK (kind IN ('s3', 'oss', 'local')),
    bucket          TEXT         NOT NULL,                         -- bucket name (e.g. "tm-state")
    region          TEXT         NOT NULL DEFAULT '',              -- bucket region (e.g. "cn-hangzhou")
    endpoint        TEXT         NOT NULL DEFAULT '',              -- custom endpoint for OSS/MinIO
    encrypt         BOOLEAN      NOT NULL DEFAULT TRUE,            -- server-side encryption
    lock_table      TEXT         NOT NULL DEFAULT '',              -- DynamoDB / OSS lock table name
    access_style    TEXT         NOT NULL DEFAULT 'oidc'           -- how Executor authenticates
                    CHECK (access_style IN ('oidc', 'aksk', 'anonymous')),
    credentials_ref TEXT         NOT NULL DEFAULT '',              -- Vault/KMS ref for aksk fallback
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,           -- exactly one row should be true
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()            -- auto-maintained by trigger
);

COMMENT ON TABLE state_backends IS 'State backend config (D6). L1 platform-wide source of truth; codegen reads this to render backend.tf, replacing the hard-coded bucket literal in doc 02.';
COMMENT ON COLUMN state_backends.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN state_backends.name IS 'Stable human name, e.g. "default-s3".';
COMMENT ON COLUMN state_backends.kind IS 'Backend type: s3|oss|local.';
COMMENT ON COLUMN state_backends.bucket IS 'Bucket name (e.g. "tm-state").';
COMMENT ON COLUMN state_backends.region IS 'Bucket region (e.g. "cn-hangzhou").';
COMMENT ON COLUMN state_backends.endpoint IS 'Custom endpoint for OSS / MinIO compatible stores.';
COMMENT ON COLUMN state_backends.encrypt IS 'Server-side encryption flag passed to terraform backend config.';
COMMENT ON COLUMN state_backends.lock_table IS 'DynamoDB (aws) or OSS lock table name for state locking.';
COMMENT ON COLUMN state_backends.access_style IS 'How Executor authenticates to the bucket: oidc|aksk|anonymous.';
COMMENT ON COLUMN state_backends.credentials_ref IS 'Vault/KMS ref for aksk fallback; never the secret itself.';
COMMENT ON COLUMN state_backends.is_default IS 'Exactly one row should be true; codegen uses the default when a stack has no env/account override.';
COMMENT ON COLUMN state_backends.created_at IS 'Record creation time.';
COMMENT ON COLUMN state_backends.updated_at IS 'Auto-maintained by set_updated_at() trigger.';

-- Enforce at most one default. A partial unique index lets us flip is_default
-- without a CHECK constraint that would block the very first insert.
CREATE UNIQUE INDEX IF NOT EXISTS uq_state_backends_single_default
    ON state_backends ((1)) WHERE is_default;
CREATE INDEX IF NOT EXISTS ix_state_backends_kind ON state_backends(kind);
CREATE TRIGGER trg_state_backends_updated_at
    BEFORE UPDATE ON state_backends FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed a default backend so a fresh install has a working backend.tf source.
-- This mirrors the previous hard-coded "tm-state" literal and keeps the
-- initial dev flow green; admins override via UPDATE.
INSERT INTO state_backends (id, name, kind, bucket, region, endpoint, encrypt, lock_table, access_style, is_default)
VALUES (0, 'default', 's3', 'tm-state', '', '', TRUE, '', 'oidc', TRUE)
ON CONFLICT (id) DO NOTHING;

-- =====================================================================
-- 3. workspaces: the platform-managed infra-repo registration (D4/D29).
--    One row per logical infra-repo (the monorepo holding all stacks).
--    The platform clones this repo and drives per-request worktrees.
-- =====================================================================
CREATE TABLE IF NOT EXISTS workspaces (
    id              BIGINT       PRIMARY KEY,                     -- snowflake ID
    name            TEXT         NOT NULL,                         -- logical name, e.g. "infra-prod"
    remote_url      TEXT         NOT NULL,                         -- bare repo URL the platform clones
    default_branch  TEXT         NOT NULL DEFAULT 'main',          -- long-lived branch codegen merges into
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),           -- record creation time
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()            -- auto-maintained by trigger
);

COMMENT ON TABLE workspaces IS 'The platform-managed infra-repo (D4). One row per logical monorepo; workspace_checkouts holds per-request worktrees.';
COMMENT ON COLUMN workspaces.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN workspaces.name IS 'Logical name, e.g. "infra-prod".';
COMMENT ON COLUMN workspaces.remote_url IS 'Bare repo URL the platform clones via go-git.';
COMMENT ON COLUMN workspaces.default_branch IS 'Long-lived branch codegen commits onto (e.g. main).';
COMMENT ON COLUMN workspaces.created_at IS 'Record creation time.';
COMMENT ON COLUMN workspaces.updated_at IS 'Auto-maintained by set_updated_at() trigger.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_workspaces_name ON workspaces(name);
CREATE TRIGGER trg_workspaces_updated_at
    BEFORE UPDATE ON workspaces FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 4. workspace_checkouts: per-request worktree + pinned_commit (D4/D21).
--    Backs the requests.pinned_commit FK target and the reconcile loop on
--    platform restart (doc 10 section 4). Each active request gets an
--    exclusive worktree so concurrent requests never clobber each other.
-- =====================================================================
CREATE TABLE IF NOT EXISTS workspace_checkouts (
    id                    BIGINT       PRIMARY KEY,               -- snowflake ID
    workspace_id          BIGINT       NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,  -- owning workspace
    node_id               TEXT         NOT NULL,                  -- executor node host name
    worktree_path         TEXT         NOT NULL,                  -- absolute path of the worktree on the node
    branch                TEXT         NOT NULL,                  -- per-request branch, e.g. "req-123"
    pinned_commit         TEXT         NOT NULL,                  -- commit SHA codegen produced (Executor checks this out)
    purpose               TEXT         NOT NULL DEFAULT 'plan_apply',  -- plan_apply | drift | import
                          CHECK (purpose IN ('plan_apply', 'drift', 'import')),
    leased_by_request_id  BIGINT       NULL REFERENCES requests(id) ON DELETE SET NULL,  -- current owning request
    leased_until          TIMESTAMPTZ  NULL,                      -- lease expiry; reconcile frees stale rows
    status                TEXT         NOT NULL DEFAULT 'active'  -- lifecycle of the checkout
                          CHECK (status IN ('active', 'released', 'stale')),
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),     -- record creation time
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now()      -- auto-maintained by trigger
);

COMMENT ON TABLE workspace_checkouts IS 'Per-request git worktree (D4/D21). Backs requests.pinned_commit ownership and the platform-restart reconcile loop (doc 10 section 4).';
COMMENT ON COLUMN workspace_checkouts.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN workspace_checkouts.workspace_id IS 'Owning workspace. FK workspaces(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN workspace_checkouts.node_id IS 'Executor node host name.';
COMMENT ON COLUMN workspace_checkouts.worktree_path IS 'Absolute path of the worktree on the node.';
COMMENT ON COLUMN workspace_checkouts.branch IS 'Per-request branch, e.g. "req-123".';
COMMENT ON COLUMN workspace_checkouts.pinned_commit IS 'Commit SHA codegen produced. Executor checks this out; drift runs reuse it.';
COMMENT ON COLUMN workspace_checkouts.purpose IS 'plan_apply | drift | import.';
COMMENT ON COLUMN workspace_checkouts.leased_by_request_id IS 'Current owning request. FK requests(id) ON DELETE SET NULL.';
COMMENT ON COLUMN workspace_checkouts.leased_until IS 'Lease expiry; reconcile loop frees stale rows (doc 10 section 4).';
COMMENT ON COLUMN workspace_checkouts.status IS 'active (in use) | released | stale (orphaned).';
COMMENT ON COLUMN workspace_checkouts.created_at IS 'Record creation time.';
COMMENT ON COLUMN workspace_checkouts.updated_at IS 'Auto-maintained by set_updated_at() trigger.';

CREATE INDEX IF NOT EXISTS ix_workspace_checkouts_workspace_id ON workspace_checkouts(workspace_id);
CREATE INDEX IF NOT EXISTS ix_workspace_checkouts_leased_by_request_id ON workspace_checkouts(leased_by_request_id);
CREATE INDEX IF NOT EXISTS ix_workspace_checkouts_status ON workspace_checkouts(status);
CREATE TRIGGER trg_workspace_checkouts_updated_at
    BEFORE UPDATE ON workspace_checkouts FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 5. stacks: generated stack identity (D29 stack.tm.hcl mirror).
--    The codegen output contract (doc 09 section 5.1) produces a stack per
--    catalog item per (tenant, env, team, space). This table persists the
--    PathGenerator outputs (repo_path, state_key, stack_id, tags) so the
--    platform can reconcile "DB metadata <-> repo dir <-> remote state key"
--    (doc 04 section 2.3, doc 10 section 4).
-- =====================================================================
CREATE TABLE IF NOT EXISTS stacks (
    id                            BIGINT       PRIMARY KEY,       -- snowflake ID
    space_id                     BIGINT       NULL REFERENCES spaces(id) ON DELETE RESTRICT,      -- optional space grouping
    catalog_item_id               BIGINT       NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,  -- source catalog item
    layer_logical_id              TEXT         NULL REFERENCES layer_logical_refs(logical_id) ON DELETE RESTRICT,  -- layer identity (D24/D26)
    layer_rule_set_version_id     INTEGER      NULL REFERENCES layer_rule_set_versions(version_id) ON DELETE RESTRICT,  -- pinned rule set (D26)
    owner_team_id                 BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,  -- team that owns this stack
    layer                         TEXT         NOT NULL,          -- layer slug, e.g. "application" (denormalized for fast filter)
    component                     TEXT         NOT NULL,          -- component slug, e.g. "rds" (PathGenerator component)
    env                           TEXT         NOT NULL,          -- env slug, e.g. "prod" (PathGenerator env)
    tenant_id                     TEXT         NOT NULL DEFAULT 'platform-default',  -- tenant slug (MVP: single tenant string)
    stack_id                      TEXT         NOT NULL,          -- PathGenerator global-unique id, e.g. "application-platform-default-team-a-rds-prod"
    repo_path                     TEXT         NOT NULL,          -- relative dir in infra-repo, e.g. "application/platform-default/team-a/rds-prod"
    state_key                     TEXT         NOT NULL,          -- remote state key, equals repo_path by default
    terramate_tags_json           JSONB        NOT NULL DEFAULT '[]',  -- tags array: ["layer:application","tenant:platform-default",...]
    state_backend_id              BIGINT       NULL REFERENCES state_backends(id) ON DELETE RESTRICT,  -- backend override (NULL = use default)
    pinned_commit                 TEXT         NOT NULL DEFAULT '',  -- last commit that touched this stack (for drift re-plan)
    migration_status              TEXT         NOT NULL DEFAULT 'stable'
                                  CHECK (migration_status IN ('stable', 'migration_pending', 'migrating', 'deprecated')),
    sunset_deadline               TIMESTAMPTZ  NULL,              -- for deprecated (Tier 3) stacks
    version                       INT          NOT NULL DEFAULT 1,  -- optimistic lock / bump on regen
    created_at                    TIMESTAMPTZ  NOT NULL DEFAULT now(),  -- record creation time
    updated_at                    TIMESTAMPTZ  NOT NULL DEFAULT now()   -- auto-maintained by trigger
);

COMMENT ON TABLE stacks IS 'Generated stack identity (D29 stack.tm.hcl mirror). Persisted PathGenerator outputs (repo_path, state_key, stack_id, tags) for reconcile of DB metadata <-> repo dir <-> remote state key.';
COMMENT ON COLUMN stacks.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN stacks.space_id IS 'Optional space grouping. FK spaces(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN stacks.catalog_item_id IS 'Source catalog item. FK catalog_items(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN stacks.layer_logical_id IS 'Layer identity (stable across rule-set versions, D24/D26). FK layer_logical_refs(logical_id).';
COMMENT ON COLUMN stacks.layer_rule_set_version_id IS 'Pinned layer rule set at creation time (D26, immutable). FK layer_rule_set_versions(version_id).';
COMMENT ON COLUMN stacks.owner_team_id IS 'Team that owns this stack. FK teams(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN stacks.layer IS 'Layer slug, e.g. "application" (denormalized for fast filter / Terramate tags).';
COMMENT ON COLUMN stacks.component IS 'Component slug, e.g. "rds" (PathGenerator component).';
COMMENT ON COLUMN stacks.env IS 'Env slug, e.g. "prod" (PathGenerator env).';
COMMENT ON COLUMN stacks.tenant_id IS 'Tenant slug. MVP: single tenant "platform-default" string (tenants table is B11 non-MVP).';
COMMENT ON COLUMN stacks.stack_id IS 'PathGenerator global-unique id, e.g. "application-platform-default-team-a-rds-prod".';
COMMENT ON COLUMN stacks.repo_path IS 'Relative dir in infra-repo, e.g. "application/platform-default/team-a/rds-prod".';
COMMENT ON COLUMN stacks.state_key IS 'Remote state key. Equals repo_path by default (D6).';
COMMENT ON COLUMN stacks.terramate_tags_json IS 'Tags array: ["layer:application","tenant:platform-default","env:prod","team:team-a"] (D29).';
COMMENT ON COLUMN stacks.state_backend_id IS 'Backend override for this stack. NULL = use state_backends.is_default row (account-per-env escape hatch).';
COMMENT ON COLUMN stacks.pinned_commit IS 'Last commit that touched this stack (for drift re-plan).';
COMMENT ON COLUMN stacks.migration_status IS 'stable | migration_pending | migrating | deprecated (D26 Tier classification).';
COMMENT ON COLUMN stacks.sunset_deadline IS 'For deprecated (Tier 3) stacks: must destroy+recreate before this date.';
COMMENT ON COLUMN stacks.version IS 'Optimistic lock; bumped on each regen.';
COMMENT ON COLUMN stacks.created_at IS 'Record creation time.';
COMMENT ON COLUMN stacks.updated_at IS 'Auto-maintained by set_updated_at() trigger.';

CREATE INDEX IF NOT EXISTS ix_stacks_space_id ON stacks(space_id);
CREATE INDEX IF NOT EXISTS ix_stacks_catalog_item_id ON stacks(catalog_item_id);
CREATE INDEX IF NOT EXISTS ix_stacks_layer_logical_id ON stacks(layer_logical_id);
CREATE INDEX IF NOT EXISTS ix_stacks_owner_team_id ON stacks(owner_team_id);
CREATE INDEX IF NOT EXISTS ix_stacks_env ON stacks(env);
CREATE INDEX IF NOT EXISTS ix_stacks_tenant_id ON stacks(tenant_id);
CREATE INDEX IF NOT EXISTS ix_stacks_stack_id ON stacks(stack_id);
CREATE INDEX IF NOT EXISTS ix_stacks_repo_path ON stacks(repo_path);
CREATE INDEX IF NOT EXISTS ix_stacks_state_backend_id ON stacks(state_backend_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_stacks_stack_id ON stacks(stack_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_stacks_repo_path_active
    ON stacks(repo_path) WHERE migration_status <> 'deprecated';
CREATE TRIGGER trg_stacks_updated_at
    BEFORE UPDATE ON stacks FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =====================================================================
-- 6. stack_dependencies: runtime cross-layer wiring (D29 after/watch).
--    Codegen materializes this from module_dependencies + the (env, tenant,
--    layer) binding when a stack is generated. Each row says "from_stack
--    depends on to_stack" and records the variable + output_key + kind so
--    codegen can render the cross-layer.tf block.
-- =====================================================================
CREATE TABLE IF NOT EXISTS stack_dependencies (
    id              BIGINT       PRIMARY KEY,                     -- snowflake ID
    from_stack_id   BIGINT       NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,  -- downstream stack (depends on)
    to_stack_id     BIGINT       NOT NULL REFERENCES stacks(id) ON DELETE RESTRICT,  -- upstream stack (depended upon)
    kind            TEXT         NOT NULL DEFAULT 'remote_state'  -- wiring kind
                    CHECK (kind IN ('remote_state', 'data_source', 'watch_only')),
    variable_name   TEXT         NOT NULL DEFAULT '',             -- downstream var that receives the upstream output
    output_key      TEXT         NOT NULL DEFAULT '',             -- upstream output key (validated vs stacks outputs)
    inject_as       TEXT         NOT NULL DEFAULT '',             -- alias for the data block in generated main.tf
    status          TEXT         NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'deprecated')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()           -- record creation time
);

COMMENT ON TABLE stack_dependencies IS 'Runtime cross-layer wiring (D29 after/watch). Codegen materializes from module_dependencies + (env, tenant, layer) binding.';
COMMENT ON COLUMN stack_dependencies.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN stack_dependencies.from_stack_id IS 'Downstream stack (depends on). FK stacks(id) ON DELETE CASCADE.';
COMMENT ON COLUMN stack_dependencies.to_stack_id IS 'Upstream stack (depended upon). FK stacks(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN stack_dependencies.kind IS 'Wiring kind: remote_state | data_source | watch_only.';
COMMENT ON COLUMN stack_dependencies.variable_name IS 'Downstream variable that receives the upstream output (e.g. "vpc_id").';
COMMENT ON COLUMN stack_dependencies.output_key IS 'Upstream output key, validated against stacks outputs.';
COMMENT ON COLUMN stack_dependencies.inject_as IS 'Alias for the data block in generated main.tf (e.g. "vpc").';
COMMENT ON COLUMN stack_dependencies.status IS 'active | deprecated.';
COMMENT ON COLUMN stack_dependencies.created_at IS 'Record creation time.';

CREATE INDEX IF NOT EXISTS ix_stack_dependencies_from_stack_id ON stack_dependencies(from_stack_id);
CREATE INDEX IF NOT EXISTS ix_stack_dependencies_to_stack_id ON stack_dependencies(to_stack_id);
CREATE INDEX IF NOT EXISTS ix_stack_dependencies_kind ON stack_dependencies(kind);

-- +goose Down

DROP INDEX IF EXISTS ix_stack_dependencies_kind;
DROP INDEX IF EXISTS ix_stack_dependencies_to_stack_id;
DROP INDEX IF EXISTS ix_stack_dependencies_from_stack_id;
DROP TABLE IF EXISTS stack_dependencies;

DROP TRIGGER IF EXISTS trg_stacks_updated_at ON stacks;
DROP INDEX IF EXISTS uq_stacks_repo_path_active;
DROP INDEX IF EXISTS uq_stacks_stack_id;
DROP INDEX IF EXISTS ix_stacks_state_backend_id;
DROP INDEX IF EXISTS ix_stacks_repo_path;
DROP INDEX IF EXISTS ix_stacks_stack_id;
DROP INDEX IF EXISTS ix_stacks_tenant_id;
DROP INDEX IF EXISTS ix_stacks_env;
DROP INDEX IF EXISTS ix_stacks_owner_team_id;
DROP INDEX IF EXISTS ix_stacks_layer_logical_id;
DROP INDEX IF EXISTS ix_stacks_catalog_item_id;
DROP INDEX IF EXISTS ix_stacks_space_id;
DROP TABLE IF EXISTS stacks;

DROP TRIGGER IF EXISTS trg_workspace_checkouts_updated_at ON workspace_checkouts;
DROP INDEX IF EXISTS ix_workspace_checkouts_status;
DROP INDEX IF EXISTS ix_workspace_checkouts_leased_by_request_id;
DROP INDEX IF EXISTS ix_workspace_checkouts_workspace_id;
DROP TABLE IF EXISTS workspace_checkouts;

DROP TRIGGER IF EXISTS trg_workspaces_updated_at ON workspaces;
DROP INDEX IF EXISTS uq_workspaces_name;
DROP TABLE IF EXISTS workspaces;

-- Remove the seeded default backend row plus any user-added rows.
DROP TRIGGER IF EXISTS trg_state_backends_updated_at ON state_backends;
DROP INDEX IF EXISTS ix_state_backends_kind;
DROP INDEX IF EXISTS uq_state_backends_single_default;
DROP TABLE IF EXISTS state_backends;

-- Re-add module_type to keep the Down path reversible. The CHECK and default
-- match the original 003_registry.sql definition so a Down then Up round trip
-- is idempotent.
ALTER TABLE modules ADD COLUMN IF NOT EXISTS module_type TEXT NOT NULL DEFAULT 'atomic'
    CHECK (module_type IN ('atomic', 'control', 'declarative'));
CREATE INDEX IF NOT EXISTS ix_modules_module_type ON modules(module_type);
COMMENT ON COLUMN modules.module_type IS 'Three-layer architecture type: atomic|control|declarative.';
