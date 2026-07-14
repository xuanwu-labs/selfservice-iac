-- 010_layers.sql: layer_logical_refs + layer_rule_set_versions + Phase 1 seed
-- + back-fill deferred FKs (bundles/catalog_items/requests reference these).
-- design.md §03 A7. Phase 1 ships a fixed 3-layer seed (global/middleware/
-- application) and one active rule-set version v1. D24/D26 dynamic layering
-- machinery (CRUD, versioning, migrations) is non-MVP.

-- +goose Up
CREATE TABLE IF NOT EXISTS layer_logical_refs (
    logical_id              TEXT         PRIMARY KEY,
    current_display_name    TEXT         NOT NULL DEFAULT '',
    notes                   TEXT         NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS layer_rule_set_versions (
    version_id      INTEGER      PRIMARY KEY,
    layers_json     JSONB        NOT NULL,    -- full layer config: name/order/path_template/depends_on/owning_team_pattern
    status          TEXT         NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'superseded', 'deprecated', 'archived')),
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_by      TEXT         NOT NULL DEFAULT '',
    superseded_at   TIMESTAMPTZ  NULL,
    superseded_by   INTEGER      NULL REFERENCES layer_rule_set_versions(version_id)
);

-- Back-fill deferred FKs now that the layer tables exist.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_bundles_layer_logical_id') THEN
        ALTER TABLE bundles ADD CONSTRAINT fk_bundles_layer_logical_id
            FOREIGN KEY (layer_logical_id) REFERENCES layer_logical_refs(logical_id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_catalog_items_layer_logical_id') THEN
        ALTER TABLE catalog_items ADD CONSTRAINT fk_catalog_items_layer_logical_id
            FOREIGN KEY (layer_logical_id) REFERENCES layer_logical_refs(logical_id) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_requests_layer_rule_set_version_id') THEN
        ALTER TABLE requests ADD CONSTRAINT fk_requests_layer_rule_set_version_id
            FOREIGN KEY (layer_rule_set_version_id) REFERENCES layer_rule_set_versions(version_id) ON DELETE RESTRICT;
    END IF;
END $$;

-- Phase 1 seed: 3 layers + 1 active rule-set v1.
-- Idempotent via ON CONFLICT (re-running migrations won't duplicate).
INSERT INTO layer_logical_refs (logical_id, current_display_name, notes, created_at) VALUES
    ('global',     'Global',     'Platform-wide infra (network, clusters, CEN). Owned by platform-ops.', now()),
    ('middleware', 'Middleware', 'Shared middleware (RDS, Redis, Kafka). DBA + middleware teams, peer at same layer.', now()),
    ('application','Application','Business application stacks. Each business team independent.', now())
ON CONFLICT (logical_id) DO NOTHING;

INSERT INTO layer_rule_set_versions (version_id, layers_json, status, is_default, created_at, created_by) VALUES (
    1,
    '[
        {"logical_id":"global","order":1,"owning_team_pattern":"platform","path_template":"global/{{.component}}-{{.tenant}}-{{.env}}","depends_on":[]},
        {"logical_id":"middleware","order":2,"owning_team_pattern":"dba|middleware","path_template":"middleware/{{.tenant}}/{{.component}}-{{.env}}","depends_on":["global"]},
        {"logical_id":"application","order":3,"owning_team_pattern":"business","path_template":"application/{{.tenant}}/{{.team}}/{{if .bundle}}{{.bundle}}/{{end}}{{.component}}-{{.env}}","depends_on":["global","middleware"]}
    ]'::jsonb,
    'active', TRUE, now(), 'system-seed'
)
ON CONFLICT (version_id) DO NOTHING;

-- +goose Down
ALTER TABLE requests DROP CONSTRAINT IF EXISTS fk_requests_layer_rule_set_version_id;
ALTER TABLE catalog_items DROP CONSTRAINT IF EXISTS fk_catalog_items_layer_logical_id;
ALTER TABLE bundles DROP CONSTRAINT IF EXISTS fk_bundles_layer_logical_id;
DROP TABLE IF EXISTS layer_rule_set_versions;
DROP TABLE IF EXISTS layer_logical_refs;
