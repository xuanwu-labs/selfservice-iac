-- 009_audit.sql: audit_logs (append-only) + outbox_events (Saga).
-- design.md §03 A8. audit_logs has ai_metadata_json (doc 17 §9.2) for AI
-- operation traceability; HMAC chain columns (prev_hash/entry_hash/signing_key_id/
-- sealed_at) are added only when compliance mode is enabled (doc 20 §2) — not
-- in MVP. outbox_events uses a 5-value status enum (doc 04 §2.8a) + event_id
-- UNIQUE for exactly-once idempotency (doc 12a IDEMP-005).

-- +goose Up
CREATE TABLE IF NOT EXISTS audit_logs (
    id                  BIGINT       PRIMARY KEY,
    actor_id            TEXT         NOT NULL DEFAULT '',
    actor_type          TEXT         NOT NULL DEFAULT 'system'
                        CHECK (actor_type IN ('human', 'ai', 'system')),
    action              TEXT         NOT NULL,
    target_type         TEXT         NOT NULL DEFAULT '',
    target_id           TEXT         NOT NULL DEFAULT '',
    before_json         JSONB        NULL,
    after_json          JSONB        NULL,
    ai_metadata_json    JSONB        NULL,   -- only when actor_type=ai (doc 17 §9.2)
    correlation_id      TEXT         NOT NULL DEFAULT '',
    occurred_at         TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_audit_logs_target ON audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS ix_audit_logs_correlation_id ON audit_logs(correlation_id);
CREATE INDEX IF NOT EXISTS ix_audit_logs_occurred_at ON audit_logs(occurred_at);
-- No updated_at trigger — append-only.

CREATE TABLE IF NOT EXISTS outbox_events (
    id              BIGINT       PRIMARY KEY,
    event_id        TEXT         NOT NULL,    -- UNIQUE, exactly-once idempotency (doc 12a IDEMP-005)
    aggregate_type  TEXT         NOT NULL,
    aggregate_id    TEXT         NOT NULL,
    event_type      TEXT         NOT NULL,
    payload_json    JSONB        NOT NULL DEFAULT '{}',
    status          TEXT         NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead-letter')),
    retry_count     INTEGER      NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ  NULL,
    correlation_id  TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at    TIMESTAMPTZ  NULL
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
