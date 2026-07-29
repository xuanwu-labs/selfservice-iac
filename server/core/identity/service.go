// Package identity implements local identity management (design D10/D10.1).
//
// Phase 1 is intentionally narrow: CRUD over the identities table + an
// idempotent BootstrapAdmin that seeds the platform-scope admin role binding
// at startup. The OIDC verification path lives in internal/auth — this package
// is the data layer identities are persisted through (and the source of
// subject_id for RBAC).
//
// Wire boundary (P3-16): internal/auth may import core/identity (internal →
// core is allowed). core/identity does NOT import internal/auth — no cycle.
package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// Provider / status / role constants. These mirror migration 015 + design D2
// and are kept here so callers do not sprinkle string literals.
const (
	ProviderLocal     = "local"
	ProviderOIDC      = "oidc"
	PrimarySourceOIDC = "oidc"

	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusMerged   = "merged"

	// Scope / role values for role_bindings (design D2).
	ScopeTypePlatform = "platform"
	ScopeTypeTeam     = "team"
	RoleAdmin         = "admin"
	RoleMember        = "member"
	RoleOwner         = "owner"

	// AdminActions is the wildcard actions list seeded for the bootstrap admin.
	// Encoded as JSONB on role_bindings.actions.
	AdminActions = `["*"]`
)

// CreateParams is the input to IdentityService.Create. ID defaults to a fresh
// snowflake when zero. Status defaults to active when empty.
type CreateParams struct {
	ExternalID    string
	DisplayName   string
	Email         string
	ProviderName  string // "local" or "oidc"; "" → "local"
	PrimarySource string // "" → "oidc"
	Status        string // "" → "active"
}

// IdentityService is the data-layer surface over identities + role_bindings.
// It wraps the pool directly for the parts sqlc does not generate yet
// (identities + role_bindings are new in migration 015 and have not been
// added to pkg/db/queries/*.sql — see Open Question in tasks.md 0.2).
type IdentityService struct {
	pool *pgxpool.Pool
}

// NewIdentityService constructs an IdentityService bound to the given pool.
func NewIdentityService(pool *pgxpool.Pool) *IdentityService {
	return &IdentityService{pool: pool}
}

// Create inserts a new identity and returns it.
func (s *IdentityService) Create(ctx context.Context, p CreateParams) (generated.Identity, error) {
	if s == nil || s.pool == nil {
		return generated.Identity{}, errors.New("identity: service not configured (nil pool)")
	}
	if p.ExternalID == "" {
		return generated.Identity{}, errors.New("identity: external_id is required")
	}
	if p.ProviderName == "" {
		p.ProviderName = ProviderLocal
	}
	if p.PrimarySource == "" {
		p.PrimarySource = PrimarySourceOIDC
	}
	if p.Status == "" {
		p.Status = StatusActive
	}

	row := generated.Identity{
		ID:            utils.GenerateID(),
		ExternalID:    p.ExternalID,
		DisplayName:   p.DisplayName,
		Email:         p.Email,
		ProviderName:  p.ProviderName,
		PrimarySource: p.PrimarySource,
		Status:        p.Status,
	}

	const sql = `INSERT INTO identities
		(id, external_id, display_name, email, provider_name, primary_source, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, external_id, display_name, email, provider_name, primary_source, status, merged_into_id, last_synced_at, created_at, updated_at`

	err := s.pool.QueryRow(ctx, sql,
		row.ID, row.ExternalID, row.DisplayName, row.Email, row.ProviderName, row.PrimarySource, row.Status,
	).Scan(
		&row.ID, &row.ExternalID, &row.DisplayName, &row.Email, &row.ProviderName, &row.PrimarySource, &row.Status,
		&row.MergedIntoID, &row.LastSyncedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return generated.Identity{}, fmt.Errorf("identity: insert identities: %w", err)
	}
	return row, nil
}

// GetByID loads an identity by its snowflake id.
func (s *IdentityService) GetByID(ctx context.Context, id int64) (generated.Identity, error) {
	if s == nil || s.pool == nil {
		return generated.Identity{}, errors.New("identity: service not configured (nil pool)")
	}
	const sql = `SELECT id, external_id, display_name, email, provider_name, primary_source, status, merged_into_id, last_synced_at, created_at, updated_at
		FROM identities WHERE id = $1`
	var row generated.Identity
	err := s.pool.QueryRow(ctx, sql, id).Scan(
		&row.ID, &row.ExternalID, &row.DisplayName, &row.Email, &row.ProviderName, &row.PrimarySource, &row.Status,
		&row.MergedIntoID, &row.LastSyncedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return generated.Identity{}, fmt.Errorf("identity: get by id %d: %w", id, err)
	}
	return row, nil
}

// GetByExternalID loads an identity by (external_id, provider_name). The
// unique index uq_identities_external_id_active is partial on status != 'merged',
// so this naturally skips merged rows.
func (s *IdentityService) GetByExternalID(ctx context.Context, externalID, provider string) (generated.Identity, error) {
	if s == nil || s.pool == nil {
		return generated.Identity{}, errors.New("identity: service not configured (nil pool)")
	}
	if provider == "" {
		provider = ProviderLocal
	}
	const sql = `SELECT id, external_id, display_name, email, provider_name, primary_source, status, merged_into_id, last_synced_at, created_at, updated_at
		FROM identities
		WHERE external_id = $1 AND provider_name = $2 AND status <> 'merged'`
	var row generated.Identity
	err := s.pool.QueryRow(ctx, sql, externalID, provider).Scan(
		&row.ID, &row.ExternalID, &row.DisplayName, &row.Email, &row.ProviderName, &row.PrimarySource, &row.Status,
		&row.MergedIntoID, &row.LastSyncedAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return generated.Identity{}, fmt.Errorf("identity: get by external_id %q/%q: %w", externalID, provider, err)
	}
	return row, nil
}

// List returns all non-merged identities ordered by created_at desc.
func (s *IdentityService) List(ctx context.Context) ([]generated.Identity, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("identity: service not configured (nil pool)")
	}
	const sql = `SELECT id, external_id, display_name, email, provider_name, primary_source, status, merged_into_id, last_synced_at, created_at, updated_at
		FROM identities WHERE status <> 'merged' ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("identity: list: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[generated.Identity])
}

// BootstrapAdmin is idempotent: it ensures an identity exists for the given
// admin external_id (creating it if missing) and that the identity carries a
// platform-scope admin role binding with actions=["*"]. Safe to call on every
// startup. Returns the identity row (whether created or pre-existing) and the
// role_binding id.
//
// The platform-scope admin binding is what EvaluateRBAC short-circuits on:
// any subject with a role=admin, scope_type=platform binding passes every
// action check (see internal/auth/rbac.go).
func (s *IdentityService) BootstrapAdmin(ctx context.Context, adminExternalID, adminDisplayName, adminEmail string) (generated.Identity, int64, error) {
	if s == nil || s.pool == nil {
		return generated.Identity{}, 0, errors.New("identity: service not configured (nil pool)")
	}
	if adminExternalID == "" {
		return generated.Identity{}, 0, errors.New("identity: bootstrap admin external_id is required")
	}

	// 1. Find or create the admin identity. The (external_id, provider_name)
	//    pair is unique among active rows; provider is fixed to "local" for the
	//    bootstrap admin since it is platform-internal, not from an OIDC IdP.
	ident, err := s.GetByExternalID(ctx, adminExternalID, ProviderLocal)
	if err != nil {
		if !isNotFound(err) {
			return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap lookup admin: %w", err)
		}
		ident, err = s.Create(ctx, CreateParams{
			ExternalID:    adminExternalID,
			DisplayName:   adminDisplayName,
			Email:         adminEmail,
			ProviderName:  ProviderLocal,
			PrimarySource: ProviderLocal,
			Status:        StatusActive,
		})
		if err != nil {
			return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap create admin: %w", err)
		}
	}

	// 2. Ensure the platform-scope admin binding exists. Use INSERT ... ON
	//    CONFLICT DO NOTHING keyed on (subject_id, scope_type, scope_id, role)
	//    so the call is idempotent across restarts. There is no unique index
	//    on that tuple yet (migration 015 only indexes subject_id and scope),
	//    so we guard with an explicit existence check inside the same tx to
	//    avoid duplicate rows when two startup goroutines race.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var bindingID int64
	const existsSQL = `SELECT id FROM role_bindings
		WHERE subject_id = $1 AND scope_type = 'platform' AND scope_id = '' AND role = 'admin'
		LIMIT 1`
	err = tx.QueryRow(ctx, existsSQL, ident.ExternalID).Scan(&bindingID)
	if err != nil && !isNotFound(err) {
		return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap check binding: %w", err)
	}
	if isNotFound(err) {
		bindingID = utils.GenerateID()
		const insertSQL = `INSERT INTO role_bindings (id, subject_id, role, scope_type, scope_id, actions)
			VALUES ($1, $2, 'admin', 'platform', '', $3)`
		if _, err := tx.Exec(ctx, insertSQL, bindingID, ident.ExternalID, AdminActions); err != nil {
			return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap insert binding: %w", err)
		}
	}

	// Touch last_synced_at so operators can see the most recent bootstrap.
	const touchSQL = `UPDATE identities SET last_synced_at = $1 WHERE id = $2 AND last_synced_at IS NULL`
	if _, err := tx.Exec(ctx, touchSQL, time.Now().UTC(), ident.ID); err != nil {
		return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap touch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return generated.Identity{}, 0, fmt.Errorf("identity: bootstrap commit: %w", err)
	}
	return ident, bindingID, nil
}

// isNotFound reports whether err is a pgx ErrNoRows. We use the exported
// sentinel rather than string-matching the error message.
func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
