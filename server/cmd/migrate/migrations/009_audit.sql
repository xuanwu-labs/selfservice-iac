-- 009_audit.sql: audit_logs (append-only) + outbox_events (Saga).
-- design.md §03 A8. audit_logs has ai_metadata_json (doc 17 §9.2) for AI
-- operation traceability; HMAC chain columns (prev_hash/entry_hash/signing_key_id/
-- sealed_at) are added only when compliance mode is enabled (doc 20 §2) — not
-- in MVP. outbox_events uses a 5-value status enum (doc 04 §2.8a) + event_id
-- UNIQUE for exactly-once idempotency (doc 12a IDEMP-005).

-- +goose Up
CREATE TABLE IF NOT EXISTS audit_logs (
    id                  BIGINT       PRIMARY KEY,                          -- snowflake ID
    actor_id            TEXT         NOT NULL DEFAULT '',                  -- who performed the action
    actor_team_id       TEXT         NOT NULL DEFAULT '',  -- proto Actor.team_id
    actor_type          TEXT         NOT NULL DEFAULT 'system'             -- proto ActorType (unspecified/human/ai/system)
                        CHECK (actor_type IN ('unspecified', 'human', 'ai', 'system')),  -- proto ActorType
    action              TEXT         NOT NULL,                             -- action verb, e.g. create_request/approve
    target_type         TEXT         NOT NULL DEFAULT '',                  -- kind of object acted on (e.g. request)
    target_id           TEXT         NOT NULL DEFAULT '',                  -- id of the object acted on
    before_json         JSONB        NULL,                                 -- pre-action snapshot of the target (diff base)
    after_json          JSONB        NULL,                                 -- post-action snapshot of the target
    ai_metadata_json    JSONB        NULL,   -- only when actor_type=ai (doc 17 §9.2)
    correlation_id      TEXT         NOT NULL DEFAULT '',                  -- trace / distributed correlation id
    occurred_at         TIMESTAMPTZ  NOT NULL DEFAULT now()                -- when the action happened (immutable)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE audit_logs IS 'Append-only audit trail (design.md §03 A8). HMAC chain columns are added only when compliance mode is enabled (doc 20 §2).';
COMMENT ON COLUMN audit_logs.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN audit_logs.actor_id IS 'Who performed the action.';
COMMENT ON COLUMN audit_logs.actor_team_id IS 'proto Actor.team_id.';
COMMENT ON COLUMN audit_logs.actor_type IS 'proto ActorType (unspecified/human/ai/system).';
COMMENT ON COLUMN audit_logs.action IS 'Action verb, e.g. create_request/approve.';
COMMENT ON COLUMN audit_logs.target_type IS 'Kind of object acted on (e.g. request).';
COMMENT ON COLUMN audit_logs.target_id IS 'id of the object acted on.';
COMMENT ON COLUMN audit_logs.before_json IS 'Pre-action snapshot of the target (diff base).';
COMMENT ON COLUMN audit_logs.after_json IS 'Post-action snapshot of the target.';
COMMENT ON COLUMN audit_logs.ai_metadata_json IS 'AI operation traceability, only set when actor_type=ai (doc 17 §9.2).';
COMMENT ON COLUMN audit_logs.correlation_id IS 'Trace / distributed correlation id.';
COMMENT ON COLUMN audit_logs.occurred_at IS 'When the action happened (immutable).';

CREATE INDEX IF NOT EXISTS ix_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS ix_audit_logs_correlation_id ON audit_logs(correlation_id);
CREATE INDEX IF NOT EXISTS ix_audit_logs_occurred_at ON audit_logs(occurred_at);
-- No updated_at trigger — append-only.

CREATE TABLE IF NOT EXISTS outbox_events (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    event_id        TEXT         NOT NULL,    -- UNIQUE, exactly-once idempotency (doc 12a IDEMP-005)
    aggregate_type  TEXT         NOT NULL,                             -- kind of aggregate this event concerns
    aggregate_id    TEXT         NOT NULL,                             -- id of the aggregate this event concerns
    event_type      TEXT         NOT NULL,                             -- event verb/type name
    payload_json    JSONB        NOT NULL DEFAULT '{}',                -- full event payload (opaque to the relay)
    status          TEXT         NOT NULL DEFAULT 'pending'            -- relay lifecycle (doc 04 §2.8a)
                    CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead-letter')),
    retry_count     INTEGER      NOT NULL DEFAULT 0,                  -- delivery attempt counter
    next_retry_at   TIMESTAMPTZ  NULL,                                -- scheduled time for the next retry
    correlation_id  TEXT         NOT NULL DEFAULT '',                  -- trace / distributed correlation id
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),               -- row creation time
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),               -- last update time (trigger-maintained)
    processed_at    TIMESTAMPTZ  NULL                                  -- when delivery finalized (succeeded/dead-letter)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE outbox_events IS 'Transactional outbox for reliable event delivery (Saga). 5-value status enum (doc 04 §2.8a); event_id UNIQUE for exactly-once idempotency (doc 12a IDEMP-005).';
COMMENT ON COLUMN outbox_events.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN outbox_events.event_id IS 'UNIQUE event id for exactly-once idempotency (doc 12a IDEMP-005).';
COMMENT ON COLUMN outbox_events.aggregate_type IS 'Kind of aggregate this event concerns.';
COMMENT ON COLUMN outbox_events.aggregate_id IS 'id of the aggregate this event concerns.';
COMMENT ON COLUMN outbox_events.event_type IS 'Event verb/type name.';
COMMENT ON COLUMN outbox_events.payload_json IS 'Full event payload (opaque to the relay).';
COMMENT ON COLUMN outbox_events.status IS 'Relay lifecycle (doc 04 §2.8a): pending|processing|succeeded|failed|dead-letter.';
COMMENT ON COLUMN outbox_events.retry_count IS 'Delivery attempt counter.';
COMMENT ON COLUMN outbox_events.next_retry_at IS 'Scheduled time for the next retry.';
COMMENT ON COLUMN outbox_events.correlation_id IS 'Trace / distributed correlation id.';
COMMENT ON COLUMN outbox_events.created_at IS 'Row creation time.';
COMMENT ON COLUMN outbox_events.updated_at IS 'Last update time (trigger-maintained).';
COMMENT ON COLUMN outbox_events.processed_at IS 'When delivery finalized (succeeded/dead-letter).';

CREATE UNIQUE INDEX IF NOT EXISTS uq_outbox_events_event_id ON outbox_events(event_id);
CREATE INDEX IF NOT EXISTS ix_outbox_events_status ON outbox_events(status);
CREATE INDEX IF NOT EXISTS ix_outbox_events_next_retry_at ON outbox_events(next_retry_at);
CREATE TRIGGER trg_outbox_events_updated_at
    BEFORE UPDATE ON outbox_events FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER IF EXISTS trg_outbox_events_updated_at ON outbox_events;
DROP INDEX IF EXISTS ix_outbox_events_next_retry_at;
DROP INDEX IF EXISTS ix_outbox_events_status;
DROP INDEX IF EXISTS uq_outbox_events_event_id;
DROP TABLE IF EXISTS outbox_events;
DROP INDEX IF EXISTS ix_audit_logs_occurred_at;
DROP INDEX IF EXISTS ix_audit_logs_correlation_id;
DROP INDEX IF EXISTS ix_audit_logs_target;
DROP TABLE IF EXISTS audit_logs;
