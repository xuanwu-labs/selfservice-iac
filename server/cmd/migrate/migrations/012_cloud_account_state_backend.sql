-- 012_cloud_account_state_backend.sql: cloud_accounts.state_backend_id.
--
-- Closes the per-env bucket isolation gap (2026-07-16 architecture review Q2).
-- Without this column the state_backends table has no env/account binding:
-- every environment shares one platform-default bucket and isolation relies
-- solely on the per-stack state_key prefix. That is acceptable for the MVP
-- single-environment flow but breaks the industry-standard "one bucket per
-- environment" model once staging/prod land.
--
-- With this column the resolution chain becomes:
--   stacks(env) -> environments(B11) -> cloud_accounts.state_backend_id
--                                            -> state_backends(bucket)
-- codegen uses it to render backend.tf with a per-env bucket, while the
-- per-stack key still comes from PathGenerator (stacks.state_key).

-- +goose Up
ALTER TABLE cloud_accounts
    ADD COLUMN IF NOT EXISTS state_backend_id BIGINT NULL REFERENCES state_backends(id) ON DELETE SET NULL;

COMMENT ON COLUMN cloud_accounts.state_backend_id IS 'Default state backend for this cloud account. NULL = use state_backends.is_default row. Enables per-env bucket isolation: environments(B11).cloud_account_id -> cloud_accounts.state_backend_id -> state_backends.bucket.';

CREATE INDEX IF NOT EXISTS ix_cloud_accounts_state_backend_id ON cloud_accounts(state_backend_id);

-- +goose Down
DROP INDEX IF EXISTS ix_cloud_accounts_state_backend_id;
ALTER TABLE cloud_accounts DROP COLUMN IF EXISTS state_backend_id;
