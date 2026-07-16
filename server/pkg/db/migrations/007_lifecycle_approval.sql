-- 007_lifecycle_approval.sql: approval_flows + approval_runs + approval_node_runs + approval_decisions.
-- design.md §03 A5. The approval engine (doc 12 §6). Dual-gate via
-- approval_runs.gate (pre-plan / pre-apply / break-glass-retroactive, D21).
-- Partial unique on (request_id, gate) WHERE status='pending' prevents
-- concurrent duplicate runs per gate (doc 12 §3 — "two independent runs").

-- +goose Up
CREATE TABLE IF NOT EXISTS approval_flows (
    id          BIGINT       PRIMARY KEY,                       -- snowflake ID
    name        TEXT         NOT NULL,                          -- human-readable flow name (unique per version)
    trigger     TEXT         NOT NULL DEFAULT '',     -- dsl_when expression
    dsl_yaml    TEXT         NOT NULL,                -- full flow YAML
    version     INTEGER      NOT NULL DEFAULT 1,                 -- immutable flow version (name+version unique)
    active      BOOLEAN      NOT NULL DEFAULT TRUE,             -- whether this flow version is selectable
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),            -- row creation time
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()             -- last update time (trigger-maintained)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE approval_flows IS 'Versioned approval flow definition parsed from DSL YAML (doc 12 §6 approval engine).';
COMMENT ON COLUMN approval_flows.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN approval_flows.name IS 'Human-readable flow name (unique per version).';
COMMENT ON COLUMN approval_flows.trigger IS 'dsl_when expression selecting when this flow runs.';
COMMENT ON COLUMN approval_flows.dsl_yaml IS 'Full flow YAML definition.';
COMMENT ON COLUMN approval_flows.version IS 'Immutable flow version (name+version unique).';
COMMENT ON COLUMN approval_flows.active IS 'Whether this flow version is selectable.';
COMMENT ON COLUMN approval_flows.created_at IS 'Row creation time.';
COMMENT ON COLUMN approval_flows.updated_at IS 'Last update time (trigger-maintained).';

CREATE UNIQUE INDEX IF NOT EXISTS uq_approval_flows_name_version
    ON approval_flows(name, version);
CREATE TRIGGER trg_approval_flows_updated_at
    BEFORE UPDATE ON approval_flows FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS approval_runs (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,  -- FK requests - request under approval
    flow_id         BIGINT       NOT NULL REFERENCES approval_flows(id) ON DELETE RESTRICT,  -- FK approval_flows - flow being executed
    gate            TEXT         NOT NULL                              -- dual-gate selector (proto ApprovalGate)
                    CHECK (gate IN ('pre_plan', 'pre_apply', 'break_glass_retroactive')),  -- proto ApprovalGate
    current_node    TEXT         NOT NULL DEFAULT '',                  -- id of the node the run is waiting on
    status          TEXT         NOT NULL DEFAULT 'pending'            -- run lifecycle (proto ApprovalRunStatus)
                    CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),  -- proto ApprovalRunStatus
    decided_by      TEXT         NOT NULL DEFAULT '',                  -- actor who closed the run (final approver/rejecter)
    decided_at      TIMESTAMPTZ  NULL,                                 -- when the run was decided/closed
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),               -- when the run started
    finished_at     TIMESTAMPTZ  NULL,                                 -- when the run finished (decided/expired)
    expires_at      TIMESTAMPTZ  NULL,                                 -- TTL - run auto-expires after this time
    version         INTEGER      NOT NULL DEFAULT 0  -- optimistic lock (DecideApproval expected_run_version)
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE approval_runs IS 'Approval run for a request at a specific dual-gate (pre-plan / pre-apply / break-glass-retroactive, D21). doc 12 §6.';
COMMENT ON COLUMN approval_runs.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN approval_runs.request_id IS 'Request under approval. FK requests(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN approval_runs.flow_id IS 'Flow being executed. FK approval_flows(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN approval_runs.gate IS 'Dual-gate selector (proto ApprovalGate): pre_plan|pre_apply|break_glass_retroactive.';
COMMENT ON COLUMN approval_runs.current_node IS 'id of the node the run is waiting on.';
COMMENT ON COLUMN approval_runs.status IS 'Run lifecycle (proto ApprovalRunStatus): pending|approved|rejected|expired.';
COMMENT ON COLUMN approval_runs.decided_by IS 'Actor who closed the run (final approver/rejecter).';
COMMENT ON COLUMN approval_runs.decided_at IS 'When the run was decided/closed.';
COMMENT ON COLUMN approval_runs.started_at IS 'When the run started.';
COMMENT ON COLUMN approval_runs.finished_at IS 'When the run finished (decided/expired).';
COMMENT ON COLUMN approval_runs.expires_at IS 'TTL - run auto-expires after this time.';
COMMENT ON COLUMN approval_runs.version IS 'Optimistic lock (DecideApproval expected_run_version).';

CREATE INDEX IF NOT EXISTS ix_approval_runs_request_id ON approval_runs(request_id);
CREATE INDEX IF NOT EXISTS ix_approval_runs_flow_id ON approval_runs(flow_id);
-- At most one pending run per (request, gate) — prevents concurrent duplicate
-- pre-apply races (doc 12 §3). Partial: allows history of completed runs.
CREATE UNIQUE INDEX IF NOT EXISTS uq_approval_runs_req_gate_pending
    ON approval_runs(request_id, gate) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS approval_node_runs (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    run_id          BIGINT       NOT NULL REFERENCES approval_runs(id) ON DELETE CASCADE,  -- FK approval_runs - parent run
    node_id         TEXT         NOT NULL,                             -- logical node id within the flow YAML
    mode            TEXT         NOT NULL                              -- decision aggregation (proto ApprovalNodeMode)
                    CHECK (mode IN ('any', 'all', 'majority', 'quorum')),  -- proto ApprovalNodeMode (quorum + required_count = count>=N)
    decided_count   INTEGER      NOT NULL DEFAULT 0,                  -- count of decisions collected so far
    required_count  INTEGER      NOT NULL DEFAULT 1,                  -- decisions needed to satisfy the node mode
    status          TEXT         NOT NULL DEFAULT 'pending'            -- node lifecycle (proto ApprovalNodeStatus)
                    CHECK (status IN ('pending', 'approved', 'rejected', 'skipped', 'timeout')),  -- proto ApprovalNodeStatus
    timeout_at      TIMESTAMPTZ  NULL                                 -- node auto-times out after this time
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE approval_node_runs IS 'Per-node execution state within an approval run. Aggregates decisions per the node mode.';
COMMENT ON COLUMN approval_node_runs.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN approval_node_runs.run_id IS 'Parent run. FK approval_runs(id) ON DELETE CASCADE.';
COMMENT ON COLUMN approval_node_runs.node_id IS 'Logical node id within the flow YAML.';
COMMENT ON COLUMN approval_node_runs.mode IS 'Decision aggregation (proto ApprovalNodeMode): any|all|majority|quorum.';
COMMENT ON COLUMN approval_node_runs.decided_count IS 'Count of decisions collected so far.';
COMMENT ON COLUMN approval_node_runs.required_count IS 'Decisions needed to satisfy the node mode (quorum: count>=N).';
COMMENT ON COLUMN approval_node_runs.status IS 'Node lifecycle (proto ApprovalNodeStatus): pending|approved|rejected|skipped|timeout.';
COMMENT ON COLUMN approval_node_runs.timeout_at IS 'Node auto-times out after this time.';

CREATE INDEX IF NOT EXISTS ix_approval_node_runs_run_id ON approval_node_runs(run_id);

-- Append-only decisions.
CREATE TABLE IF NOT EXISTS approval_decisions (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    node_run_id     BIGINT       NOT NULL REFERENCES approval_node_runs(id) ON DELETE RESTRICT,  -- FK approval_node_runs - node decided on
    approver_id     TEXT         NOT NULL,            -- MVP dangling string (identities is B4)
    decision        TEXT         NOT NULL                              -- proto ApprovalDecision (no abstain)
                    CHECK (decision IN ('approved', 'rejected')),  -- proto ApprovalDecision (no abstain)
    comment         TEXT         NOT NULL DEFAULT '',                  -- optional rationale text from the approver
    decided_at      TIMESTAMPTZ  NOT NULL DEFAULT now()                -- when the approver submitted the decision
);

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE approval_decisions IS 'Append-only individual approver decision. IDEMP-004: an approver can decide once per node.';
COMMENT ON COLUMN approval_decisions.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN approval_decisions.node_run_id IS 'Node decided on. FK approval_node_runs(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN approval_decisions.approver_id IS 'MVP dangling string (identities is B4).';
COMMENT ON COLUMN approval_decisions.decision IS 'proto ApprovalDecision (no abstain): approved|rejected.';
COMMENT ON COLUMN approval_decisions.comment IS 'Optional rationale text from the approver.';
COMMENT ON COLUMN approval_decisions.decided_at IS 'When the approver submitted the decision.';

CREATE INDEX IF NOT EXISTS ix_approval_decisions_node_run_id ON approval_decisions(node_run_id);
-- IDEMP-004: same approver can't decide twice on the same node.
CREATE UNIQUE INDEX IF NOT EXISTS uq_approval_decisions_node_approver
    ON approval_decisions(node_run_id, approver_id);

-- +goose Down
DROP INDEX IF EXISTS uq_approval_decisions_node_approver;
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
