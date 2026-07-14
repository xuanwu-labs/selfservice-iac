-- 005_lifecycle_requests.sql: requests + request_events.
-- design.md §03 A4. The request lifecycle state machine (19 statuses, doc 00 §5
-- + doc 12a) and append-only event trail. plan_artifact_id FK is added by
-- 006 after plan_artifacts exists.

-- +goose Up
CREATE TABLE IF NOT EXISTS requests (
    id                          BIGINT       PRIMARY KEY,
    catalog_item_id             BIGINT       NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,
    bundle_id                   BIGINT       NULL REFERENCES bundles(id) ON DELETE RESTRICT,
    env_id                      TEXT         NOT NULL,                          -- MVP dangling string (envs table is B11)
    tenant_id                   TEXT         NOT NULL,                          -- MVP dangling string (tenants table is B11)
    team_id                     BIGINT       NOT NULL REFERENCES teams(id) ON DELETE RESTRICT,
    requester_id                TEXT         NOT NULL,                          -- MVP dangling string (identities table is B4)
    kind                        TEXT         NOT NULL DEFAULT 'standard'
                                CHECK (kind IN ('standard', 'drift-remediation', 'legacy-import', 'maintenance-apply')),
    source                      TEXT         NOT NULL DEFAULT 'web'
                                CHECK (source IN ('web', 'cli', 'cicd', 'ai')),
    status                      TEXT         NOT NULL DEFAULT 'submitted'
                                CHECK (status IN (
                                    'submitted', 'generating', 'pending-admission', 'planning', 'plan-ready',
                                    'pending-approval', 'applying', 'reconciling', 'succeeded', 'reconcile-pending',
                                    'rejected', 'cancelled', 'expired', 'failed-retryable', 'failed-terminal',
                                    'waiting-manual', 'blocked-policy', 'blocked-state-health', 'paused-drift')),
    current_stage               TEXT         NOT NULL DEFAULT '',
    form_values_json            JSONB        NOT NULL DEFAULT '{}',
    form_hash                   TEXT         NOT NULL,
    resolved_params_json        JSONB        NULL,                              -- doc 08 provenance (per-var source/rank)
    idempotency_key             TEXT         NOT NULL,                          -- sha256(actor+catalog+form_hash+24h_window)
    pinned_commit               TEXT         NULL,
    plan_artifact_id            BIGINT       NULL,                              -- FK added by 006
    cost_estimate_cents         BIGINT       NOT NULL DEFAULT 0,
    cost_currency               TEXT         NOT NULL DEFAULT 'USD',
    correlation_id              TEXT         NOT NULL DEFAULT '',
    retry_count                 INTEGER      NOT NULL DEFAULT 0,               -- doc 12 reject.terminal (>=3)
    version                     INTEGER      NOT NULL DEFAULT 0,               -- doc 00 §5 optimistic lock
    layer_rule_set_version_id   INTEGER      NULL,                             -- FK added by 010_layers
    created_at                  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_requests_catalog_item_id ON requests(catalog_item_id);
CREATE INDEX IF NOT EXISTS ix_requests_bundle_id ON requests(bundle_id);
CREATE INDEX IF NOT EXISTS ix_requests_team_id ON requests(team_id);
CREATE INDEX IF NOT EXISTS ix_requests_plan_artifact_id ON requests(plan_artifact_id);
CREATE INDEX IF NOT EXISTS ix_requests_layer_rule_set_version_id ON requests(layer_rule_set_version_id);
CREATE INDEX IF NOT EXISTS ix_requests_status ON requests(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_requests_idempotency_key ON requests(idempotency_key);
CREATE TRIGGER trg_requests_updated_at
    BEFORE UPDATE ON requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Append-only event trail (state machine trajectory). No updated_at — rows are
-- never modified after insert.
CREATE TABLE IF NOT EXISTS request_events (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    event_type      TEXT         NOT NULL
                    CHECK (event_type IN ('state_transition', 'log', 'error', 'approval', 'hook')),
    stage           TEXT         NOT NULL DEFAULT '',
    from_status     TEXT         NULL,
    to_status       TEXT         NULL,
    actor_id        TEXT         NOT NULL DEFAULT '',
    actor_type      TEXT         NOT NULL DEFAULT 'system'
                    CHECK (actor_type IN ('human', 'ai', 'system')),
    message         TEXT         NOT NULL DEFAULT '',
    correlation_id  TEXT         NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);
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
DROP INDEX IF EXISTS ix_requests_bundle_id;
DROP INDEX IF EXISTS ix_requests_catalog_item_id;
DROP TABLE IF EXISTS requests;
