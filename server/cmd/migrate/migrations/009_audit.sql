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
