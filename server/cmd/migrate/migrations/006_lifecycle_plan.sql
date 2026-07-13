-- 006_lifecycle_plan.sql: plan_artifacts + gate_results.
-- design.md §03 A4. plan_artifacts is independent (not merged into executor_runs)
-- and carries ALL plan/apply version-consistency fields (doc 09 §5.2 + doc 12
-- invariant 0) — this is the D21 plan/apply decoupling safety precondition.
-- Also back-fills requests.plan_artifact_id FK now that this table exists.

-- +goose Up
CREATE TABLE IF NOT EXISTS plan_artifacts (
    id                        BIGINT       PRIMARY KEY,
    request_id                BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    status                    TEXT         NOT NULL DEFAULT 'active'
                              CHECK (status IN ('active', 'superseded', 'expired', 'orphan')),
    plan_hash                 TEXT         NOT NULL,
    storage_uri               TEXT         NOT NULL,
    sha256                    TEXT         NOT NULL,
    size_bytes                BIGINT       NOT NULL DEFAULT 0,
    pinned_commit             TEXT         NOT NULL,                            -- apply must check out same commit
    toolchain_profile_hash    TEXT         NOT NULL,                            -- apply TF version must match
    provider_lock_hash        TEXT         NOT NULL,                            -- apply .terraform.lock.hcl must match
    tf_version_sha256         TEXT         NOT NULL,                            -- apply terraform binary must match
    stack_id                  TEXT         NOT NULL,                            -- PathGenerator output (doc 09 §5.2)
    state_key                 TEXT         NOT NULL,                            -- PathGenerator output
    resources_to_add          INTEGER      NOT NULL DEFAULT 0,
    resources_to_change       INTEGER      NOT NULL DEFAULT 0,
    resources_to_destroy      INTEGER      NOT NULL DEFAULT 0,
    cost_estimate_cents       BIGINT       NOT NULL DEFAULT 0,
    expires_at                TIMESTAMPTZ  NULL,                                -- TTL: apply must verify now < expires_at (doc 12)
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_plan_artifacts PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS ix_plan_artifacts_request_id ON plan_artifacts(request_id);

-- Back-fill the FK from requests to plan_artifacts (column added in 005).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_requests_plan_artifact_id'
    ) THEN
        ALTER TABLE requests
            ADD CONSTRAINT fk_requests_plan_artifact_id
            FOREIGN KEY (plan_artifact_id) REFERENCES plan_artifacts(id) ON DELETE SET NULL;
    END IF;
END $$;

-- gate_results: policy/OPA evaluation outcomes per request (proto GateResult).
CREATE TABLE IF NOT EXISTS gate_results (
    id              BIGINT       PRIMARY KEY,
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,
    gate_id         TEXT         NOT NULL,
    passed          BOOLEAN      NOT NULL,
    policy          TEXT         NOT NULL DEFAULT '',
    message         TEXT         NOT NULL DEFAULT '',
    severity        TEXT         NOT NULL DEFAULT 'info'
                    CHECK (severity IN ('info', 'warning', 'error', 'critical')),
    evaluated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_gate_results PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS ix_gate_results_request_id ON gate_results(request_id);

-- +goose Down
DROP INDEX IF EXISTS ix_gate_results_request_id;
DROP TABLE IF EXISTS gate_results;
ALTER TABLE requests DROP CONSTRAINT IF EXISTS fk_requests_plan_artifact_id;
DROP INDEX IF EXISTS ix_plan_artifacts_request_id;
DROP TABLE IF EXISTS plan_artifacts;
