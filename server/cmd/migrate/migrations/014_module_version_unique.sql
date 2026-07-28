-- 014_module_version_unique.sql: UNIQUE constraint on module_versions(module_id, version)
--
-- Prevents duplicate registration of the same (module, version) pair (Gap 4).
-- Without this, a retry of RegisterModule(git_source, version) creates a second
-- module_versions row, breaking idempotency and is_current semantics.
--
-- This is the DB-level backstop for idempotency (P1-1 from W1-03 review,
-- scoped to W2 for full idempotency-key handling, but the UNIQUE constraint is
-- cheap insurance that belongs in the schema now).

-- +goose Up

CREATE UNIQUE INDEX IF NOT EXISTS uq_module_versions_module_version
    ON module_versions(module_id, version);

COMMENT ON INDEX uq_module_versions_module_version IS 'Prevents duplicate (module_id, version) registration. DB-level idempotency backstop.';

-- +goose Down

DROP INDEX IF EXISTS uq_module_versions_module_version;
