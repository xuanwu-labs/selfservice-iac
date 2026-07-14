-- 006_lifecycle_plan.sql: plan_artifacts + gate_results.
-- design.md §03 A4. plan_artifacts is independent (not merged into executor_runs)
-- and carries ALL plan/apply version-consistency fields (doc 09 §5.2 + doc 12
-- invariant 0) — this is the D21 plan/apply decoupling safety precondition.
-- Also back-fills requests.plan_artifact_id FK now that this table exists.

-- +goose Up
CREATE TABLE IF NOT EXISTS plan_artifacts (
    id                        BIGINT       PRIMARY KEY,                       -- snowflake ID
    request_id                BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,  -- FK requests - owning request
    status                    TEXT         NOT NULL DEFAULT 'ready'           -- artifact lifecycle (proto ArtifactStatus)
                              CHECK (status IN ('ready', 'expired', 'consumed', 'superseded')),  -- proto ArtifactStatus
    plan_hash                 TEXT         NOT NULL,                          -- content hash of the rendered plan (integrity key)
    storage_uri               TEXT         NOT NULL,                          -- object-store location of the plan file
    sha256                    TEXT         NOT NULL,                          -- sha256 of the stored plan artifact bytes
    size_bytes                BIGINT       NOT NULL DEFAULT 0,                -- size of the stored artifact in bytes
    pinned_commit             TEXT         NOT NULL,                            -- apply must check out same commit
    toolchain_profile_hash    TEXT         NOT NULL,                            -- apply TF version must match
    provider_lock_hash        TEXT         NOT NULL,                            -- apply .terraform.lock.hcl must match
    tf_version_sha256         TEXT         NOT NULL,                            -- apply terraform binary must match
    stack_id                  TEXT         NOT NULL,                            -- PathGenerator output (doc 09 §5.2)
    state_key                 TEXT         NOT NULL,                            -- PathGenerator output
    resources_to_add          INTEGER      NOT NULL DEFAULT 0,                -- plan summary: resources to add
    resources_to_change       INTEGER      NOT NULL DEFAULT 0,                -- plan summary: resources to change
    resources_to_destroy      INTEGER      NOT NULL DEFAULT 0,                -- plan summary: resources to destroy
    cost_estimate_cents       BIGINT       NOT NULL DEFAULT 0,                -- estimated cost in cents for this plan
    expires_at                TIMESTAMPTZ  NULL,                                -- TTL: apply must verify now < expires_at (doc 12)
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now()             -- artifact creation time
);
CREATE INDEX IF NOT EXISTS ix_plan_artifacts_request_id ON plan_artifacts(request_id);

-- Back-fill the FK from requests to plan_artifacts (column added in 005).
-- +goose StatementBegin
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
-- +goose StatementEnd

-- gate_results: policy/OPA evaluation outcomes per request (proto GateResult).
CREATE TABLE IF NOT EXISTS gate_results (
    id              BIGINT       PRIMARY KEY,                          -- snowflake ID
    request_id      BIGINT       NOT NULL REFERENCES requests(id) ON DELETE RESTRICT,  -- FK requests - evaluated request
    gate_id         TEXT         NOT NULL,                             -- logical gate identifier (e.g. OPA rule name)
    passed          BOOLEAN      NOT NULL,                             -- whether the gate passed
    policy          TEXT         NOT NULL DEFAULT '',                  -- policy/rule reference that was evaluated
    message         TEXT         NOT NULL DEFAULT '',                  -- human-readable evaluation message
    severity        TEXT         NOT NULL DEFAULT 'warning'            -- proto GateSeverity
                    CHECK (severity IN ('unspecified', 'error', 'warning')),  -- proto GateSeverity
    evaluated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()                -- when the gate was evaluated
);
CREATE INDEX IF NOT EXISTS ix_gate_results_request_id ON gate_results(request_id);

-- +goose Down
DROP INDEX IF EXISTS ix_gate_results_request_id;
DROP TABLE IF EXISTS gate_results;
ALTER TABLE requests DROP CONSTRAINT IF EXISTS fk_requests_plan_artifact_id;
DROP INDEX IF EXISTS ix_plan_artifacts_request_id;
DROP TABLE IF EXISTS plan_artifacts;
