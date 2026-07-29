-- 015_identity_rbac.sql: identities + role_bindings (D10/D23 RBAC foundation)
--
-- P0-2 fix (W3 review): migration 009 only created audit_logs + outbox_events.
-- identities + role_bindings tables were never created (requests.requester_id
-- was "MVP dangling string, identities table is B4"). This migration creates
-- them so W3 IdentityService + RBAC can function.
--
-- Schema authority: docs/04 §2.7 (identities/role_bindings) + docs/05 §5.

-- +goose Up

CREATE TABLE IF NOT EXISTS identities (
    id              BIGINT       PRIMARY KEY,
    external_id     TEXT         NOT NULL,
    display_name    TEXT         NOT NULL DEFAULT '',
    email           TEXT         NOT NULL DEFAULT '',
    provider_name   TEXT         NOT NULL DEFAULT '',
    primary_source  TEXT         NOT NULL DEFAULT 'oidc',
    status          TEXT         NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'disabled', 'merged')),
    merged_into_id  BIGINT       NULL REFERENCES identities(id) ON DELETE SET NULL,
    last_synced_at  TIMESTAMPTZ  NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE identities IS 'User identities (D10/D10.1). Phase 1: local + single OIDC issuer. Phase 2: SCIM/飞书/钉钉 multi-source.';
COMMENT ON COLUMN identities.external_id IS 'Stable external ID from OIDC provider (sub claim). Unique per provider.';
COMMENT ON COLUMN identities.provider_name IS 'OIDC provider name (oidc_providers.name, Phase 2). Phase 1 = "local" or "oidc".';
COMMENT ON COLUMN identities.status IS 'active=enabled, disabled=revoked, merged=consolidated into another identity.';
COMMENT ON COLUMN identities.merged_into_id IS 'If status=merged, points to the surviving identity.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_identities_external_id_active ON identities(external_id, provider_name) WHERE status != 'merged';

CREATE TRIGGER trg_identities_updated_at
    BEFORE UPDATE ON identities FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE IF NOT EXISTS role_bindings (
    id              BIGINT       PRIMARY KEY,
    subject_id      TEXT         NOT NULL,
    role            TEXT         NOT NULL,
    scope_type      TEXT         NOT NULL
                    CHECK (scope_type IN ('platform', 'team', 'project', 'space', 'stack', 'layer')),
    scope_id        TEXT         NOT NULL DEFAULT '',
    actions         JSONB        NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE role_bindings IS 'RBAC bindings (D10). subject_id = identity external_id or team slug. Phase 1: admin/member/owner roles.';
COMMENT ON COLUMN role_bindings.subject_id IS 'Identity external_id or team slug (e.g. "admin", "user@example.com").';
COMMENT ON COLUMN role_bindings.role IS 'Role: admin (wildcard), member (read+request), owner (read+request+approve+reject).';
COMMENT ON COLUMN role_bindings.scope_type IS 'platform=global, team/project/space/stack/layer=scoped.';
COMMENT ON COLUMN role_bindings.scope_id IS 'Entity ID within scope_type. platform scope = empty string.';
COMMENT ON COLUMN role_bindings.actions IS 'Allowed actions JSONB array. admin = ["*"]. member = ["read","request"]. owner = ["read","request","approve","reject"].';

CREATE INDEX IF NOT EXISTS ix_role_bindings_subject ON role_bindings(subject_id);
CREATE INDEX IF NOT EXISTS ix_role_bindings_scope ON role_bindings(scope_type, scope_id);

CREATE TRIGGER trg_role_bindings_updated_at
    BEFORE UPDATE ON role_bindings FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- +goose Down

DROP TRIGGER IF EXISTS trg_role_bindings_updated_at ON role_bindings;
DROP TRIGGER IF EXISTS trg_identities_updated_at ON identities;

DROP TABLE IF EXISTS role_bindings;
DROP TABLE IF EXISTS identities;
