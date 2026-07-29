// Package audit implements the audit_logs DB writer (design D5).
//
// core/audit.AuditLogger is the durable audit sink: a row in audit_logs for
// each auditable action. It is wired as an events.EventHandler on the bus so
// any subsystem that Publish'es an event can produce an audit row by
// translating it to an AuditEntry. Direct callers (handlers, services) may
// also call Log explicitly for actions that bypass the bus.
//
// Field shape mirrors migration 009 exactly. Note migration 009 has
// actor_team_id TEXT (proto Actor.team_id as a string, since teams.id is a
// snowflake but the actor may carry a slug / logical id); AuditLogger stores
// it as a string to match. id is a snowflake generated via utils.GenerateID.
package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/core/events"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
)

// ActorType values allowed by the audit_logs.actor_type CHECK constraint
// (migration 009). Proto ActorType maps 1:1 to these strings.
const (
	ActorTypeUnspecified = "unspecified"
	ActorTypeHuman       = "human"
	ActorTypeAI          = "ai"
	ActorTypeSystem      = "system"
)

// AuditEntry is the input to AuditLogger.Log. Field names mirror the
// audit_logs columns (snake-case in SQL, camelCase here per Go convention).
//
// ID and OccurredAt default to a fresh snowflake and now() respectively when
// left zero — callers normally do not set them. CorrelationID is read from
// ctx via events.CorrelationIDFromContext when left empty.
type AuditEntry struct {
	ID             int64     // snowflake; 0 → generated
	ActorID        string    // identity external_id (e.g. "admin", "alice@example.com")
	ActorTeamID    string    // proto Actor.team_id (string, matches audit_logs.actor_team_id TEXT)
	ActorType      string    // one of ActorType* constants
	Action         string    // verb, e.g. "create_request", "approve", "apply"
	TargetType     string    // kind of object, e.g. "request", "stack"
	TargetID       string    // id of the object acted on
	BeforeJSON     []byte    // pre-action snapshot (may be nil)
	AfterJSON      []byte    // post-action snapshot (may be nil)
	AIMetadataJSON []byte    // only set when ActorType=ActorTypeAI
	CorrelationID  string    // trace id; "" → read from ctx
	OccurredAt     time.Time // zero → time.Now().UTC()
}

// AuditLogger writes AuditEntry rows into audit_logs.
type AuditLogger struct {
	pool *pgxpool.Pool
}

// NewAuditLogger constructs an AuditLogger bound to the given pool.
func NewAuditLogger(pool *pgxpool.Pool) *AuditLogger {
	return &AuditLogger{pool: pool}
}

// insertSQL is the static INSERT — all values are positional parameters, no
// string interpolation. Column order matches the VALUES list.
const insertSQL = `INSERT INTO audit_logs (
	id, actor_id, actor_team_id, actor_type, action, target_type, target_id,
	before_json, after_json, ai_metadata_json, correlation_id, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

// Log writes one audit row. ID/OccurredAt/CorrelationID are defaulted when
// zero. Validation guards the actor_type CHECK constraint up front so callers
// get an InvalidArgument-flavored error instead of a raw PG violation.
func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error {
	if l == nil || l.pool == nil {
		return errors.New("audit: logger not configured (nil pool)")
	}
	if entry.ActorType == "" {
		entry.ActorType = ActorTypeSystem
	}
	if !isValidActorType(entry.ActorType) {
		return fmt.Errorf("audit: invalid actor_type %q (want one of unspecified|human|ai|system)", entry.ActorType)
	}
	if entry.Action == "" {
		return errors.New("audit: action is required")
	}
	if entry.ID == 0 {
		entry.ID = utils.GenerateID()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}
	if entry.CorrelationID == "" {
		entry.CorrelationID = events.CorrelationIDFromContext(ctx)
	}

	_, err := l.pool.Exec(ctx, insertSQL,
		entry.ID,
		entry.ActorID,
		entry.ActorTeamID,
		entry.ActorType,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		entry.BeforeJSON,
		entry.AfterJSON,
		entry.AIMetadataJSON,
		entry.CorrelationID,
		entry.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("audit: insert audit_logs: %w", err)
	}
	return nil
}

// AsEventHandler adapts the logger to the events.EventHandler signature. The
// returned handler expects event.Payload to contain the AuditEntry fields
// under well-known keys (see entryFromPayload). It never returns an error —
// audit failures are non-fatal inside the bus, so we surface them via the
// returned handler only when the bus is being driven directly (e.g. tests).
// In normal operation the bus swallows errors from handlers.
func (l *AuditLogger) AsEventHandler() events.EventHandler {
	return func(ctx context.Context, event events.Event) error {
		entry, ok := entryFromPayload(event)
		if !ok {
			// Not an audit-shaped event — ignore quietly.
			return nil
		}
		if entry.CorrelationID == "" {
			entry.CorrelationID = event.CorrelationID
		}
		return l.Log(ctx, entry)
	}
}

// entryFromPayload translates an Event into an AuditEntry. Recognised payload
// keys (all optional except Action): actor_id, actor_team_id, actor_type,
// action, target_type, target_id, before_json, after_json, ai_metadata_json.
// Returns ok=false when there is no "action" key — that means the publisher
// did not intend this event to be audited.
func entryFromPayload(event events.Event) (AuditEntry, bool) {
	if event.Payload == nil {
		return AuditEntry{}, false
	}
	getStr := func(k string) string {
		if v, ok := event.Payload[k].(string); ok {
			return v
		}
		return ""
	}
	getBytes := func(k string) []byte {
		switch v := event.Payload[k].(type) {
		case []byte:
			return v
		case string:
			return []byte(v)
		default:
			return nil
		}
	}
	action := getStr("action")
	if action == "" {
		return AuditEntry{}, false
	}
	return AuditEntry{
		ActorID:        getStr("actor_id"),
		ActorTeamID:    getStr("actor_team_id"),
		ActorType:      getStr("actor_type"),
		Action:         action,
		TargetType:     getStr("target_type"),
		TargetID:       getStr("target_id"),
		BeforeJSON:     getBytes("before_json"),
		AfterJSON:      getBytes("after_json"),
		AIMetadataJSON: getBytes("ai_metadata_json"),
	}, true
}

// isValidActorType guards the audit_logs.actor_type CHECK constraint.
func isValidActorType(s string) bool {
	switch s {
	case ActorTypeUnspecified, ActorTypeHuman, ActorTypeAI, ActorTypeSystem:
		return true
	}
	return false
}

// ProviderSet binds NewAuditLogger for wire. core packages inject *AuditLogger
// directly. The handler wiring (AsEventHandler → bus.Register) is done at app
// assembly time.
var ProviderSet = wire.NewSet(NewAuditLogger)
