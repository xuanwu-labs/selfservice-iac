-- 007_lifecycle_approval.sql: approval_flows + approval_runs + approval_node_runs + approval_decisions.
-- design.md §03 A5. The approval engine (doc 12 §6). Dual-gate via
-- approval_runs.gate (pre-plan / pre-apply / break-glass-retroactive, D21).
-- Partial unique on (request_id, gate) WHERE status='pending' prevents
-- concurrent duplicate runs per gate (doc 12 §3 — "two independent runs").

-- +goose Up
CREATE TABLE IF NOT EXISTS approval_flows (
    id          BIGINT       PRIMARY KEY,
    name        TEXT         NOT NULL,
    trigger     TEXT         NOT NULL DEFAULT '',     -- dsl_when expression
    dsl_yaml    TEXT         NOT NULL,                -- full flow YAML
    version     INTEGER      NOT NULL DEFAULT 1,
    active      BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_approval_flows PRIMARY KEY (id)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_approval_flows_name_version
    ON approval_flows(name, version);
CREATE TRIGGER trg_approval_flows_updated_at
    BEFORE UPDATE ON approval_flows FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS approval_runs (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    flow_id         BIGINT       NOT NULL REFERENCES approval_flows(id) ON DELETE RESTRICT,
    gate            TEXT         NOT NULL
                    CHECK (gate IN ('pre-plan', 'pre-apply', 'break-glass-retroactive')),
    current_node    TEXT         NOT NULL DEFAULT '',
    status          TEXT         NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'timeout', 'cancelled')),
    decided_by      TEXT         NOT NULL DEFAULT '',
    decided_at      TIMESTAMPTZ  NULL,
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ  NULL,
    expires_at      TIMESTAMPTZ  NULL,
    CONSTRAINT pk_approval_runs PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS ix_approval_runs_request_id ON approval_runs(request_id);
CREATE INDEX IF NOT EXISTS ix_approval_runs_flow_id ON approval_runs(flow_id);
-- At most one pending run per (request, gate) — prevents concurrent duplicate
-- pre-apply races (doc 12 §3). Partial: allows history of completed runs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_approval_runs_req_gate_pending
    ON approval_runs(request_id, gate) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS approval_node_runs (
    id              BIGINT       PRIMARY KEY,
    run_id          BIGINT       NOT NULL REFERENCES approval_runs(id) ON DELETE CASCADE,
    node_id         TEXT         NOT NULL,
    mode            TEXT         NOT NULL
                    CHECK (mode IN ('any', 'all', 'majority', 'count>=N')),  -- doc 12 §2.3
    decided_count   INTEGER      NOT NULL DEFAULT 0,
    required_count  INTEGER      NOT NULL DEFAULT 1,
    status          TEXT         NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'timeout')),
    timeout_at      TIMESTAMPTZ  NULL,
    CONSTRAINT pk_approval_node_runs PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS ix_approval_node_runs_run_id ON approval_node_runs(run_id);

-- Append-only decisions.
CREATE TABLE IF NOT EXISTS approval_decisions (
    id              BIGINT       PRIMARY KEY,
    node_run_id     BIGINT       NOT NULL REFERENCES approval_node_runs(id) ON DELETE RESTRICT,
    approver_id     TEXT         NOT NULL,            -- MVP dangling string (identities is B4)
    decision        TEXT         NOT NULL
                    CHECK (decision IN ('approve', 'reject', 'abstain')),
    comment         TEXT         NOT NULL DEFAULT '',
    decided_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_approval_decisions PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS ix_approval_decisions_node_run_id ON approval_decisions(node_run_id);

-- +goose Down
DROP INDEX IF EXISTS ix_approval_decisions_node_run_id;
DROP TABLE IF EXISTS approval_decisions;
DROP INDEX IF EXISTS ix_approval_node_runs_run_id;
DROP TABLE IF EXISTS approval_node_runs;
DROP INDEX IF EXISTS uq_approval_runs_req_gate_pending;
DROP INDEX IF EXISTS ix_approval_runs_flow_id;
DROP INDEX IF EXISTS ix_approval_runs_request_id;
DROP TABLE IF EXISTS approval_runs;
DROP TRIGGER IF EXISTS trg_approval_flows_updated_at ON approval_flows;
DROP INDEX IF EXISTS uq_approval_flows_name_version;
DROP TABLE IF EXISTS approval_flows;
