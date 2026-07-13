-- schema.sql: sqlc schema source. DO NOT EDIT table structure by hand —
-- this is a DDL projection of migrations/*.sql (Up blocks only) that sqlc
-- parses to generate Go types. Sync from migrations when they change.
--
-- Why a mirror instead of pointing sqlc at migrations/: sqlc parses both
-- -- +goose Up and -- +goose Down as plain SQL comments, so feeding it the
-- migration files directly would execute DROP after CREATE and fail. The
-- DO $$ ... $$ procedural FK back-fills are also not parseable by sqlc.
--
-- Topological order (referenced tables first). The requests <-> plan_artifacts
-- cycle is broken by leaving plan_artifact_id as a bare BIGINT (no inline FK)
-- — sqlc still generates the column; the runtime FK is added by migration 006.

-- 000_utils: set_updated_at() trigger function (not needed for sqlc type gen).

-- Layer tables first (referenced by bundles/catalog_items/requests).
CREATE TABLE layer_logical_refs (
    logical_id              TEXT         PRIMARY KEY,
    current_display_name    TEXT         NOT NULL DEFAULT '',
    notes                   TEXT         NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE layer_rule_set_versions (
    version_id      INTEGER      PRIMARY KEY,
    layers_json     JSONB        NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'active',
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by      TEXT         NOT NULL DEFAULT '',
    superseded_at   TIMESTAMPTZ,
    superseded_by   INTEGER      REFERENCES layer_rule_set_versions(version_id)
);

CREATE TABLE teams (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    slug        TEXT         NOT NULL,
    kind        TEXT         NOT NULL,
    status      TEXT         NOT NULL DEFAULT 'active',
    tags_json   JSONB        NOT NULL DEFAULT '{}',
    policy_json JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE projects (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    team_id     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE bundles (
    id                BIGINT       PRIMARY KEY,
    name              TEXT         NOT NULL,
    project_id        BIGINT       NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    layer_logical_id  TEXT         REFERENCES layer_logical_refs(logical_id),
    repo_path         TEXT         NOT NULL,
    tags_json         JSONB        NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE modules (
    id              BIGINT       PRIMARY KEY,
    name            TEXT         NOT NULL,
    git_source      TEXT         NOT NULL,
    provider        TEXT         NOT NULL,
    layer           TEXT         NOT NULL,
    owner_team_id   BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    status          TEXT         NOT NULL DEFAULT 'pending_validation',
    description     TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE module_versions (
    id                      BIGINT       PRIMARY KEY,
    module_id               BIGINT       NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    version                 TEXT         NOT NULL,
    commit_sha              TEXT         NOT NULL,
    providers_json          JSONB        NOT NULL DEFAULT '{}',
    variables_contract_json JSONB        NOT NULL DEFAULT '{}',
    is_current              BOOLEAN      NOT NULL DEFAULT FALSE,
    registered_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE module_dependencies (
    id                  BIGINT       PRIMARY KEY,
    module_version_id   BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    variable_name       TEXT         NOT NULL,
    depends_on_layer    TEXT         NOT NULL,
    depends_on_module   TEXT         NOT NULL,
    output_key          TEXT         NOT NULL,
    required            BOOLEAN      NOT NULL DEFAULT FALSE,
    description         TEXT         NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE catalog_items (
    id                          BIGINT       PRIMARY KEY,
    module_version_id           BIGINT       NOT NULL REFERENCES module_versions(id) ON DELETE RESTRICT,
    display_name                TEXT         NOT NULL,
    description                 TEXT         NOT NULL DEFAULT '',
    category                    TEXT         NOT NULL DEFAULT '',
    status                      TEXT         NOT NULL DEFAULT 'draft',
    form_schema_json            JSONB        NOT NULL DEFAULT '{}',
    defaults_json               JSONB        NOT NULL DEFAULT '{}',
    cardinality                 TEXT         NOT NULL DEFAULT 'single',
    instance_key                TEXT         NOT NULL DEFAULT '',
    per_instance_fields_json    JSONB        NOT NULL DEFAULT '{}',
    shared_fields_json          JSONB        NOT NULL DEFAULT '{}',
    layer_logical_id            TEXT         REFERENCES layer_logical_refs(logical_id),
    stack_grouping              TEXT         NOT NULL DEFAULT 'per-component',
    owner_team_id               BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    default_tags_json           JSONB        NOT NULL DEFAULT '{}',
    user_allowed_tag_keys_json  JSONB        NOT NULL DEFAULT '[]',
    visibility_json             JSONB        NOT NULL DEFAULT '[]',
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                  TIMESTAMPTZ
);

-- plan_artifacts before requests (requests references it). The reverse
-- (plan_artifacts.request_id -> requests) creates a cycle; plan_artifact_id
-- on requests is a bare BIGINT here (no inline FK) — runtime FK added by
-- migration 006. sqlc still generates the column.
CREATE TABLE plan_artifacts (
    id                        BIGINT       PRIMARY KEY,
    request_id                BIGINT       NOT NULL,
    status                    TEXT         NOT NULL DEFAULT 'active',
    plan_hash                 TEXT         NOT NULL,
    storage_uri               TEXT         NOT NULL,
    sha256                    TEXT         NOT NULL,
    size_bytes                BIGINT       NOT NULL DEFAULT 0,
    pinned_commit             TEXT         NOT NULL,
    toolchain_profile_hash    TEXT         NOT NULL,
    provider_lock_hash        TEXT         NOT NULL,
    tf_version_sha256         TEXT         NOT NULL,
    stack_id                  TEXT         NOT NULL,
    state_key                 TEXT         NOT NULL,
    resources_to_add          INTEGER      NOT NULL DEFAULT 0,
    resources_to_change       INTEGER      NOT NULL DEFAULT 0,
    resources_to_destroy      INTEGER      NOT NULL DEFAULT 0,
    cost_estimate_cents       BIGINT       NOT NULL DEFAULT 0,
    expires_at                TIMESTAMPTZ,
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE requests (
    id                          BIGINT       PRIMARY KEY,
    catalog_item_id             BIGINT       NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,
    bundle_id                   BIGINT       REFERENCES bundles(id) ON DELETE RESTRICT,
    env_id                      TEXT         NOT NULL,
    tenant_id                   TEXT         NOT NULL,
    team_id                     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    requester_id                TEXT         NOT NULL,
    kind                        TEXT         NOT NULL DEFAULT 'standard',
    source                      TEXT         NOT NULL DEFAULT 'web',
    status                      TEXT         NOT NULL DEFAULT 'submitted',
    current_stage               TEXT         NOT NULL DEFAULT '',
    form_values_json            JSONB        NOT NULL DEFAULT '{}',
    form_hash                   TEXT         NOT NULL,
    resolved_params_json        JSONB,
    idempotency_key             TEXT         NOT NULL,
    pinned_commit               TEXT,
    plan_artifact_id            BIGINT,
    cost_estimate_cents         BIGINT       NOT NULL DEFAULT 0,
    cost_currency               TEXT         NOT NULL DEFAULT 'USD',
    correlation_id              TEXT         NOT NULL DEFAULT '',
    retry_count                 INTEGER      NOT NULL DEFAULT 0,
    version                     INTEGER      NOT NULL DEFAULT 0,
    layer_rule_set_version_id   INTEGER      REFERENCES layer_rule_set_versions(version_id),
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Now the runtime FK for plan_artifacts.request_id (cycle resolution).
ALTER TABLE plan_artifacts ADD CONSTRAINT fk_plan_artifacts_request_id
    FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE RESTRICT;

CREATE TABLE request_events (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    event_type      TEXT         NOT NULL,
    stage           TEXT         NOT NULL DEFAULT '',
    from_status     TEXT,
    to_status       TEXT,
    actor_id        TEXT         NOT NULL DEFAULT '',
    actor_type      TEXT         NOT NULL DEFAULT 'system',
    message         TEXT         NOT NULL DEFAULT '',
    correlation_id  TEXT         NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE gate_results (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    gate_id         TEXT         NOT NULL,
    passed          BOOLEAN      NOT NULL,
    policy          TEXT         NOT NULL DEFAULT '',
    message         TEXT         NOT NULL DEFAULT '',
    severity        TEXT         NOT NULL DEFAULT 'info',
    evaluated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE approval_flows (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    trigger     TEXT         NOT NULL DEFAULT '',
    dsl_yaml    TEXT         NOT NULL,
    version     INTEGER      NOT NULL DEFAULT 1,
    active      BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE approval_runs (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    flow_id         BIGINT       NOT NULL REFERENCES approval_flows(id) ON DELETE RESTRICT,
    gate            TEXT         NOT NULL,
    current_node    TEXT         NOT NULL DEFAULT '',
    status          TEXT         NOT NULL DEFAULT 'pending',
    decided_by      TEXT         NOT NULL DEFAULT '',
    decided_at      TIMESTAMPTZ,
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ
);

CREATE TABLE approval_node_runs (
    id              BIGINT       PRIMARY KEY,
    run_id          BIGINT       NOT NULL REFERENCES approval_runs(id) ON DELETE CASCADE,
    node_id         TEXT         NOT NULL,
    mode            TEXT         NOT NULL,
    decided_count   INTEGER      NOT NULL DEFAULT 0,
    required_count  INTEGER      NOT NULL DEFAULT 1,
    status          TEXT         NOT NULL DEFAULT 'pending',
    timeout_at      TIMESTAMPTZ
);

CREATE TABLE approval_decisions (
    id              BIGINT       PRIMARY KEY,
    node_run_id     BIGINT       NOT NULL REFERENCES approval_node_runs(id) ON DELETE RESTRICT,
    approver_id     TEXT         NOT NULL,
    decision        TEXT         NOT NULL,
    comment         TEXT         NOT NULL DEFAULT '',
    decided_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE cloud_accounts (
    id                      BIGINT       PRIMARY KEY,
    provider                TEXT         NOT NULL,
    account_id              TEXT         NOT NULL,
    alias                   TEXT         NOT NULL DEFAULT '',
    display_name            TEXT         NOT NULL DEFAULT '',
    status                  TEXT         NOT NULL DEFAULT 'active',
    default_region          TEXT         NOT NULL DEFAULT '',
    regions_json            JSONB        NOT NULL DEFAULT '[]',
    credentials_ref         TEXT         NOT NULL DEFAULT '',
    billing_enabled         BOOLEAN      NOT NULL DEFAULT FALSE,
    default_team_id         BIGINT       REFERENCES teams(id) ON DELETE SET NULL,
    tags_json               JSONB        NOT NULL DEFAULT '{}',
    bootstrap_status        TEXT         NOT NULL DEFAULT 'none',
    oidc_trust_configured   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
    id                  BIGINT       PRIMARY KEY,
    actor_id            TEXT         NOT NULL DEFAULT '',
    actor_type          TEXT         NOT NULL DEFAULT 'system',
    action              TEXT         NOT NULL,
    target_type         TEXT         NOT NULL DEFAULT '',
    target_id           TEXT         NOT NULL DEFAULT '',
    before_json         JSONB,
    after_json          JSONB,
    ai_metadata_json    JSONB,
    correlation_id      TEXT         NOT NULL DEFAULT '',
    occurred_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id              BIGINT       PRIMARY KEY,
    event_id        TEXT         NOT NULL,
    aggregate_type  TEXT         NOT NULL,
    aggregate_id    TEXT         NOT NULL,
    event_type      TEXT         NOT NULL,
    payload_json    JSONB        NOT NULL DEFAULT '{}',
    status          TEXT         NOT NULL DEFAULT 'pending',
    retry_count     INTEGER      NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    correlation_id  TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ
);
