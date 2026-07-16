-- 005_lifecycle_requests.sql: requests + request_events.
-- design.md §03 A4. The request lifecycle state machine (19 statuses, doc 00 §5
-- + doc 12a) and append-only event trail. plan_artifact_id FK is added by
-- 006 after plan_artifacts exists.

-- +goose Up
CREATE TABLE IF NOT EXISTS requests (
    id                          BIGINT       PRIMARY KEY,                       -- snowflake ID
    catalog_item_id             BIGINT       NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,  -- FK catalog_items - what is being requested
    space_id                   BIGINT       NULL REFERENCES spaces(id) ON DELETE RESTRICT,  -- FK spaces - request grouping (nullable for single item)
    env_id                      TEXT         NOT NULL,                          -- MVP dangling string (envs table is B11)
    tenant_id                   TEXT         NOT NULL,                          -- MVP dangling string (tenants table is B11)
    team_id                     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,  -- FK teams - owning team
    requester_id                TEXT         NOT NULL,                          -- MVP dangling string (identities table is B4)
    kind                        TEXT         NOT NULL DEFAULT 'standard'        -- proto RequestKind: standard/drift_remediation/legacy_import/maintenance_apply
                                CHECK (kind IN ('standard', 'drift_remediation', 'legacy_import', 'maintenance_apply')),
    source                      TEXT         NOT NULL DEFAULT 'web'             -- proto RequestSource: web/cli/cicd/ai/gateway
                                CHECK (source IN ('web', 'cli', 'cicd', 'ai', 'gateway')),
    status                      TEXT         NOT NULL DEFAULT 'submitted'       -- lifecycle state (proto RequestStatus)
                                CHECK (status IN (
                                    'submitted', 'generating', 'pending_admission', 'planning', 'plan_ready',
                                    'pending_approval', 'applying', 'reconciling', 'succeeded', 'reconcile_pending',
                                    'rejected', 'cancelled', 'expired', 'failed_retryable', 'failed_terminal',
                                    'waiting_manual',
                                    'blocked_policy', 'blocked_state_health', 'paused_drift')),  -- 19 values aligned to proto RequestStatus
    current_stage               TEXT         NOT NULL DEFAULT '',               -- human-readable current pipeline stage label
    form_values_json            JSONB        NOT NULL DEFAULT '{}',             -- submitted form values keyed by field id
    form_hash                   TEXT         NOT NULL,                          -- sha256 of form_values_json - deterministic input identity
    resolved_params_json        JSONB        NULL,                              -- doc 08 provenance (per-var source/rank)
    source_context_json         JSONB        NOT NULL DEFAULT '{}',            -- proto CreateRequestRequest.source_context
    idempotency_key             TEXT         NOT NULL,                          -- sha256(actor+catalog+form_hash+24h_window)
    pinned_commit               TEXT         NULL,                              -- git commit sha the request is pinned to
    plan_artifact_id            BIGINT       NULL,                              -- FK added by 006
    cost_estimate_cents         BIGINT       NOT NULL DEFAULT 0,                -- estimated cost in cents (plan output)
    cost_currency               TEXT         NOT NULL DEFAULT 'USD',            -- ISO 4217 currency code for cost_estimate_cents
    correlation_id              TEXT         NOT NULL DEFAULT '',               -- trace / distributed correlation id
    retry_count                 INTEGER      NOT NULL DEFAULT 0,               -- doc 12 reject.terminal (>=3)
    version                     INTEGER      NOT NULL DEFAULT 0,               -- doc 00 §5 optimistic lock
    layer_rule_set_version_id   INTEGER      NULL,                             -- FK added by 010_layers
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),            -- row creation time
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now()             -- last update time (trigger-maintained)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE requests IS 'Lifecycle request state machine (19 statuses, doc 00 §5 + doc 12a). The central entity for provisioning.';
COMMENT ON COLUMN requests.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN requests.catalog_item_id IS 'What is being requested. FK catalog_items(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN requests.space_id IS 'Request grouping (nullable for single item). FK spaces(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN requests.env_id IS 'MVP dangling string (envs table is B11).';
COMMENT ON COLUMN requests.tenant_id IS 'MVP dangling string (tenants table is B11).';
COMMENT ON COLUMN requests.team_id IS 'Owning team. FK teams(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN requests.requester_id IS 'MVP dangling string (identities table is B4).';
COMMENT ON COLUMN requests.kind IS 'proto RequestKind: standard|drift_remediation|legacy_import|maintenance_apply.';
COMMENT ON COLUMN requests.source IS 'proto RequestSource: web|cli|cicd|ai|gateway.';
COMMENT ON COLUMN requests.status IS 'Lifecycle state (proto RequestStatus). 19 values incl. blocked_policy/blocked_state_health/paused_drift.';
COMMENT ON COLUMN requests.current_stage IS 'Human-readable current pipeline stage label.';
COMMENT ON COLUMN requests.form_values_json IS 'Submitted form values keyed by field id.';
COMMENT ON COLUMN requests.form_hash IS 'sha256 of form_values_json - deterministic input identity.';
COMMENT ON COLUMN requests.resolved_params_json IS 'doc 08 provenance: per-variable source/rank.';
COMMENT ON COLUMN requests.source_context_json IS 'proto CreateRequestRequest.source_context.';
COMMENT ON COLUMN requests.idempotency_key IS 'sha256(actor+catalog+form_hash+24h_window). UNIQUE.';
COMMENT ON COLUMN requests.pinned_commit IS 'Git commit sha the request is pinned to.';
COMMENT ON COLUMN requests.plan_artifact_id IS 'FK added by 006 (plan_artifacts).';
COMMENT ON COLUMN requests.cost_estimate_cents IS 'Estimated cost in cents (plan output).';
COMMENT ON COLUMN requests.cost_currency IS 'ISO 4217 currency code for cost_estimate_cents.';
COMMENT ON COLUMN requests.correlation_id IS 'Trace / distributed correlation id.';
COMMENT ON COLUMN requests.retry_count IS 'doc 12 reject.terminal counter (terminal when >=3).';
COMMENT ON COLUMN requests.version IS 'doc 00 §5 optimistic lock.';
COMMENT ON COLUMN requests.layer_rule_set_version_id IS 'FK added by 010_layers (layer_rule_set_versions).';
COMMENT ON COLUMN requests.created_at IS 'Row creation time.';
COMMENT ON COLUMN requests.updated_at IS 'Last update time (trigger-maintained).';

CREATE INDEX IF NOT EXISTS ix_requests_catalog_item_id ON requests(catalog_item_id);
CREATE INDEX IF NOT EXISTS ix_requests_space_id ON requests(space_id);
CREATE INDEX IF NOT EXISTS ix_requests_team_id ON requests(team_id);
CREATE INDEX IF NOT EXISTS ix_requests_plan_artifact_id ON requests(plan_artifact_id);
CREATE INDEX IF NOT EXISTS ix_requests_layer_rule_set_version_id ON requests(layer_rule_set_version_id);
CREATE INDEX IF NOT EXISTS ix_requests_status ON requests(status);
CREATE INDEX IF NOT EXISTS ix_requests_requester_id ON requests(requester_id);  -- ListRequests filter
CREATE UNIQUE INDEX IF NOT EXISTS uq_requests_idempotency_key ON requests(idempotency_key);
CREATE TRIGGER trg_requests_updated_at
    BEFORE UPDATE ON requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only event trail (state machine trajectory). No updated_at — rows are
-- never modified after insert.
CREATE TABLE IF NOT EXISTS request_events (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,  -- FK requests - parent request
    event_type      TEXT         NOT NULL                              -- proto RequestEvent.event_type
                    CHECK (event_type IN ('state_transition', 'log', 'error', 'approval', 'hook')),
    stage           TEXT         NOT NULL DEFAULT '',                  -- pipeline stage at event time
    from_status     TEXT         NULL,                                 -- previous status (state_transition only)
    to_status       TEXT         NULL,                                 -- new status (state_transition only)
    actor_id        TEXT         NOT NULL DEFAULT '',                  -- who/what triggered the event
    actor_team_id   TEXT         NOT NULL DEFAULT '',  -- proto Actor.team_id
    actor_type      TEXT         NOT NULL DEFAULT 'system'             -- proto ActorType (unspecified/human/ai/system)
                    CHECK (actor_type IN ('unspecified', 'human', 'ai', 'system')),  -- proto ActorType
    message         TEXT         NOT NULL DEFAULT '',                  -- human-readable event detail
    correlation_id  TEXT         NOT NULL DEFAULT '',                  -- trace / distributed correlation id
    occurred_at     TIMESTAMPTZ  NOT NULL DEFAULT now()                -- when the event happened (immutable)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE request_events IS 'Append-only event trail (state machine trajectory). Rows are never modified after insert.';
COMMENT ON COLUMN request_events.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN request_events.request_id IS 'Parent request. FK requests(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN request_events.event_type IS 'proto RequestEvent.event_type: state_transition|log|error|approval|hook.';
COMMENT ON COLUMN request_events.stage IS 'Pipeline stage at event time.';
COMMENT ON COLUMN request_events.from_status IS 'Previous status (state_transition only).';
COMMENT ON COLUMN request_events.to_status IS 'New status (state_transition only).';
COMMENT ON COLUMN request_events.actor_id IS 'Who/what triggered the event.';
COMMENT ON COLUMN request_events.actor_team_id IS 'proto Actor.team_id.';
COMMENT ON COLUMN request_events.actor_type IS 'proto ActorType: unspecified|human|ai|system.';
COMMENT ON COLUMN request_events.message IS 'Human-readable event detail.';
COMMENT ON COLUMN request_events.correlation_id IS 'Trace / distributed correlation id.';
COMMENT ON COLUMN request_events.occurred_at IS 'When the event happened (immutable).';

CREATE INDEX IF NOT EXISTS ix_request_events_request_id ON request_events(request_id);
CREATE INDEX IF NOT EXISTS ix_request_events_occurred_at ON request_events(occurred_at);

-- +goose Down
DROP INDEX IF EXISTS ix_request_events_occurred_at;
DROP INDEX IF EXISTS ix_request_events_request_id;
DROP TABLE IF EXISTS request_events;
DROP TRIGGER IF EXISTS trg_requests_updated_at ON requests;
DROP INDEX IF EXISTS uq_requests_idempotency_key;
DROP INDEX IF EXISTS ix_requests_status;
DROP INDEX IF EXISTS ix_requests_layer_rule_set_version_id;
DROP INDEX IF EXISTS ix_requests_plan_artifact_id;
DROP INDEX IF EXISTS ix_requests_team_id;
DROP INDEX IF EXISTS ix_requests_space_id;
DROP INDEX IF EXISTS ix_requests_catalog_item_id;
DROP TABLE IF EXISTS requests;
