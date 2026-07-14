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

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE plan_artifacts IS 'Immutable plan artifact carrying ALL plan/apply version-consistency fields (doc 09 §5.2 + doc 12 invariant 0). D21 plan/apply decoupling safety precondition.';
COMMENT ON COLUMN plan_artifacts.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN plan_artifacts.request_id IS 'Owning request. FK requests(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN plan_artifacts.status IS 'Artifact lifecycle (proto ArtifactStatus): ready|expired|consumed|superseded.';
COMMENT ON COLUMN plan_artifacts.plan_hash IS 'Content hash of the rendered plan (integrity key).';
COMMENT ON COLUMN plan_artifacts.storage_uri IS 'Object-store location of the plan file.';
COMMENT ON COLUMN plan_artifacts.sha256 IS 'sha256 of the stored plan artifact bytes.';
COMMENT ON COLUMN plan_artifacts.size_bytes IS 'Size of the stored artifact in bytes.';
COMMENT ON COLUMN plan_artifacts.pinned_commit IS 'Apply must check out the same commit.';
COMMENT ON COLUMN plan_artifacts.toolchain_profile_hash IS 'Apply TF version must match.';
COMMENT ON COLUMN plan_artifacts.provider_lock_hash IS 'Apply .terraform.lock.hcl must match.';
COMMENT ON COLUMN plan_artifacts.tf_version_sha256 IS 'Apply terraform binary must match.';
COMMENT ON COLUMN plan_artifacts.stack_id IS 'PathGenerator output (doc 09 §5.2).';
COMMENT ON COLUMN plan_artifacts.state_key IS 'PathGenerator output.';
COMMENT ON COLUMN plan_artifacts.resources_to_add IS 'Plan summary: resources to add.';
COMMENT ON COLUMN plan_artifacts.resources_to_change IS 'Plan summary: resources to change.';
COMMENT ON COLUMN plan_artifacts.resources_to_destroy IS 'Plan summary: resources to destroy.';
COMMENT ON COLUMN plan_artifacts.cost_estimate_cents IS 'Estimated cost in cents for this plan.';
COMMENT ON COLUMN plan_artifacts.expires_at IS 'TTL: apply must verify now < expires_at (doc 12).';
COMMENT ON COLUMN plan_artifacts.created_at IS 'Artifact creation time.';

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

-- DB-level column comments (visible in psql \d, DataGrip, DBeaver).
COMMENT ON TABLE gate_results IS 'Policy/OPA evaluation outcomes per request (proto GateResult).';
COMMENT ON COLUMN gate_results.id IS 'Snowflake ID (app-generated BIGINT, no DB autoincrement).';
COMMENT ON COLUMN gate_results.request_id IS 'Evaluated request. FK requests(id) ON DELETE RESTRICT.';
COMMENT ON COLUMN gate_results.gate_id IS 'Logical gate identifier (e.g. OPA rule name).';
COMMENT ON COLUMN gate_results.passed IS 'Whether the gate passed.';
COMMENT ON COLUMN gate_results.policy IS 'Policy/rule reference that was evaluated.';
COMMENT ON COLUMN gate_results.message IS 'Human-readable evaluation message.';
COMMENT ON COLUMN gate_results.severity IS 'proto GateSeverity: unspecified|error|warning.';
COMMENT ON COLUMN gate_results.evaluated_at IS 'When the gate was evaluated.';

CREATE INDEX IF NOT EXISTS ix_gate_results_request_id ON gate_results(request_id);

-- +goose Down
DROP INDEX IF EXISTS ix_gate_results_request_id;
DROP TABLE IF EXISTS gate_results;
ALTER TABLE requests DROP CONSTRAINT IF EXISTS fk_requests_plan_artifact_id;
DROP INDEX IF EXISTS ix_plan_artifacts_request_id;
DROP TABLE IF EXISTS plan_artifacts;
