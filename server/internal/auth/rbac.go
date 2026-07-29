// Package auth rbac.go: Phase 1 RBAC evaluation (design D2).
//
// EvaluateRBAC reads role_bindings rows and decides whether subjectID may
// perform action in (scopeType, scopeID). Phase 1 supports three roles:
//
//	admin  → actions=["*"]                (any action allowed, anywhere)
//	member → actions=["read","request"]   (read + submit requests)
//	owner  → actions=["read","request","approve","reject"]  (team approver)
//
// The wildcard check is short-circuited: if the subject has ANY binding whose
// role=admin at scope_type=platform, every action is allowed (the platform
// admin supersedes team-scoped roles). Otherwise we collect the actions JSONB
// arrays from the matching bindings and check membership.
//
// Bindings match when subject_id = subjectID AND scope_type is one of:
//   - 'platform' (always matches, no scope_id restriction), OR
//   - the requested scope_type AND scope_id = scopeID
//
// so a team-scoped owner binding authorizes actions inside that team, while a
// platform admin binding authorizes actions everywhere.
package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Scope + role constants (mirrors identity package — duplicated here to keep
// internal/auth from importing core/identity and forming a wire cycle).
const (
	rbacScopePlatform  = "platform"
	rbacActionWildcard = "*"
)

// EvaluateRBAC decides whether subjectID may perform action on
// (scopeType, scopeID). Returns (allowed, reason). reason is a short
// human-readable string explaining the decision; on denial it is safe to
// return to the caller (no internal detail leakage).
//
// pool must be a live *pgxpool.Pool connected to the platform DB. The query
// is read-only.
func EvaluateRBAC(
	ctx context.Context,
	pool *pgxpool.Pool,
	subjectID string,
	action string,
	scopeType string,
	scopeID string,
) (bool, string) {
	if pool == nil {
		return false, "rbac: pool not configured"
	}
	if subjectID == "" {
		return false, "rbac: subject id required"
	}
	if action == "" {
		return false, "rbac: action required"
	}

	// Pull all bindings that could authorize this call: same subject, and
	// scope either platform (global) or exactly (scopeType, scopeID).
	// Ordering doesn't matter — we either find a wildcard or scan actions.
	const q = `SELECT role, scope_type, scope_id, actions
		FROM role_bindings
		WHERE subject_id = $1
		  AND (
			scope_type = 'platform'
			OR (scope_type = $2 AND scope_id = $3)
		  )`
	rows, err := pool.Query(ctx, q, subjectID, scopeType, scopeID)
	if err != nil {
		return false, fmt.Sprintf("rbac: query bindings: %v", err)
	}
	defer rows.Close()

	type binding struct {
		role      string
		scopeType string
		scopeID   string
		actions   []string
	}
	var hits []binding
	for rows.Next() {
		var b binding
		var actionsJSON []byte
		if err := rows.Scan(&b.role, &b.scopeType, &b.scopeID, &actionsJSON); err != nil {
			return false, fmt.Sprintf("rbac: scan binding: %v", err)
		}
		b.actions = decodeActions(actionsJSON)
		hits = append(hits, b)
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Sprintf("rbac: iterate bindings: %v", err)
	}
	if len(hits) == 0 {
		return false, "rbac: no matching role binding"
	}

	// 1. Platform admin short-circuit. If any binding is role=admin at
	//    scope=platform, allow unconditionally — this is the bootstrap admin.
	for _, b := range hits {
		if b.scopeType == rbacScopePlatform && b.role == "admin" {
			return true, "rbac: platform admin (wildcard)"
		}
	}

	// 2. Wildcard in any matching actions array.
	for _, b := range hits {
		for _, a := range b.actions {
			if a == rbacActionWildcard {
				return true, fmt.Sprintf("rbac: wildcard via %s/%s", b.scopeType, b.role)
			}
		}
	}

	// 3. Exact action match in any matching actions array.
	for _, b := range hits {
		for _, a := range b.actions {
			if a == action {
				return true, fmt.Sprintf("rbac: action %q allowed via %s/%s", action, b.scopeType, b.role)
			}
		}
	}

	return false, fmt.Sprintf("rbac: action %q not granted by any matching binding", action)
}

// decodeActions parses a role_bindings.actions JSONB array. Returns an empty
// slice on any error so a malformed row never accidentally grants permission
// (fail-closed).
func decodeActions(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		// Fail closed: treat an unparseable actions blob as empty so a corrupt
		// row cannot grant unintended permission.
		return nil
	}
	return out
}

// SubjectFromContext is the canonical way to read the authenticated subject
// id inside a handler. The Connect Auth interceptor stores it after a
// successful OIDC verify (see middleware/connect.go).
type subjectKey struct{}

// WithSubject returns ctx annotated with the subject id (typically the
// identity external_id resolved from the OIDC sub claim).
func WithSubject(ctx context.Context, subjectID string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subjectID)
}

// SubjectFromContext returns the subject id stored via WithSubject, or "".
func SubjectFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(subjectKey{}).(string); ok {
		return v
	}
	return ""
}
