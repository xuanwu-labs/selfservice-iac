-- 000_utils.sql: shared trigger function for updated_at auto-maintenance.
--
-- sqlc does not manage audit columns (unlike GORM/sqlboiler hooks). The
-- industry pattern (Brandur / sqlc consensus) is a PG trigger that sets
-- updated_at = now() on every UPDATE, so the application layer never has to
-- pass it. Every business table that has an updated_at column MUST attach
-- this trigger:
--
--   CREATE TRIGGER trg_<table>_updated_at
--       BEFORE UPDATE ON <table>
--       FOR EACH ROW EXECUTE FUNCTION set_updated_at();
--
-- See design.md §2.2 (platform-db-schema).

-- +goose Up
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
