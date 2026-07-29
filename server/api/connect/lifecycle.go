// Package connect lifecycle.go implements LifecycleServiceHandler (design D1).
//
// Each RPC is a THIN wrapper: proto ↔ Go conversion + persistence via the
// pool + a call to orchestrator.Pipeline / ApprovalService. Business logic
// lives in the orchestrator (W2-06); this handler does not invent new flows.
//
// RPC → backend mapping (P0-1 corrected against the actual proto):
//
//	CreateRequest          → INSERT request + Pipeline.Execute
//	GetRequest             → SELECT request by id
//	ListRequests           → SELECT requests (filters + pagination)
//	ListRequestEvents      → SELECT request_events by request_id
//	CancelRequest          → state-machine transition to cancelled
//	StartPlan              → Pipeline.Execute (resume from current status)
//	GetArtifact            → SELECT plan_artifacts by id
//	EvaluateGate           → Phase 1 pass-through (no-op gate)
//	ListPendingApprovals   → SELECT approval_runs WHERE status='pending'
//	GetApprovalRun         → SELECT approval_run + nodes + decisions
//	DecideApproval         → ApprovalService.Approve / Reject (W2-06, not a new Engine)
//	StartApply             → Pipeline.Execute (resume into applying)
package connect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xuanwu-labs/selfservice-iac/server/core/events"
	"github.com/xuanwu-labs/selfservice-iac/server/core/orchestrator"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/auth"
	pkgerrors "github.com/xuanwu-labs/selfservice-iac/server/internal/errors"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/common"
	lifecyclev1 "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/lifecycle"
	lifecyclev1connect "github.com/xuanwu-labs/selfservice-iac/server/internal/proto/platform/v1/lifecycle/lifecyclev1connect"
	"github.com/xuanwu-labs/selfservice-iac/server/internal/utils"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// Compile-time assertion: LifecycleHandler satisfies the generated interface.
var _ lifecyclev1connect.LifecycleServiceHandler = (*LifecycleHandler)(nil)

// LifecycleHandler implements the LifecycleService Connect RPC. It is
// constructed once (wire) and called per request.
//
// Phase 1 (D1, D4): the handler is a thin wrapper. Persistence goes through
// the pool directly (sqlc queries for requests/approvals/plan_artifacts have
// not been generated yet); lifecycle transitions delegate to
// orchestrator.Pipeline / ApprovalService.
type LifecycleHandler struct {
	lifecyclev1connect.UnimplementedLifecycleServiceHandler

	pool     *pgxpool.Pool
	pipeline *orchestrator.Pipeline
	approval *orchestrator.ApprovalService
	queries  *generated.Queries
}

// NewLifecycleHandler constructs a LifecycleHandler. The pool, pipeline, and
// approval service are wire-injected; queries is derived from the pool so the
// handler can use both generated sqlc helpers and ad-hoc SQL.
func NewLifecycleHandler(
	pool *pgxpool.Pool,
	pipeline *orchestrator.Pipeline,
	approval *orchestrator.ApprovalService,
) *LifecycleHandler {
	return &LifecycleHandler{
		pool:     pool,
		pipeline: pipeline,
		approval: approval,
		queries:  generated.New(pool),
	}
}

// ============================================================
// --- Request CRUD ---
// ============================================================

// CreateRequest inserts a new request row and kicks the Pipeline. Per P1-5,
// the call is idempotent within a 24h window keyed on
// sha256(actor + catalog_item_id + form_hash + day): a replay returns the
// existing request without re-submitting.
func (h *LifecycleHandler) CreateRequest(
	ctx context.Context,
	req *connect.Request[lifecyclev1.CreateRequestRequest],
) (*connect.Response[lifecyclev1.CreateRequestResponse], error) {
	in := req.Msg

	catalogItemID, err := parseSnowflake("catalog_item_id", in.CatalogItemId)
	if err != nil {
		return nil, err
	}
	teamID, err := parseSnowflake("team_id", in.TeamId)
	if err != nil {
		return nil, err
	}
	var spaceID *int64
	if in.SpaceId != "" {
		sid, err := parseSnowflake("space_id", in.SpaceId)
		if err != nil {
			return nil, err
		}
		spaceID = &sid
	}

	formJSON, formHash, err := encodeFormValues(in.FormValues)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_SCHEMA_INVALID, connect.CodeInvalidArgument,
			"encode form values: %v", err)
	}

	// Subject for idempotency + requester_id attribution. Read from ctx
	// (set by the Connect Auth interceptor); fall back to "" when no auth.
	subject := auth.SubjectFromContext(ctx)
	idempotencyKey := buildIdempotencyKey(subject, in.CatalogItemId, formHash)

	// Idempotency replay: if a request with this key exists, return it.
	if existing, ok, err := h.getRequestByIdempotencyKey(ctx, idempotencyKey); err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"check idempotency: %v", err)
	} else if ok {
		return connect.NewResponse(&lifecyclev1.CreateRequestResponse{
			Request:       dbRequestToProto(&existing),
			CorrelationId: existing.CorrelationID,
		}), nil
	}

	// Insert.
	row := generated.Request{
		ID:             utils.GenerateID(),
		CatalogItemID:  catalogItemID,
		SpaceID:        spaceID,
		EnvID:          in.EnvId,
		TenantID:       in.TenantId,
		TeamID:         teamID,
		RequesterID:    subject,
		Kind:           "standard",
		Source:         sourceEnumToDB(in.Source),
		Status:         orchestrator.StatusSubmitted,
		CurrentStage:   "",
		FormValuesJson: formJSON,
		FormHash:       formHash,
		IdempotencyKey: idempotencyKey,
		CostCurrency:   "USD",
		Version:        0,
		CorrelationID:  correlationIDFrom(ctx),
	}
	if err := h.insertRequest(ctx, row); err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"insert request: %v", err)
	}

	// Phase 1 (D4): synchronous Pipeline.Execute drives submitted → plan_ready
	// (→ pending_approval). Errors here surface to the caller — the row is
	// still persisted in its terminal-failure status for inspection.
	if err := h.pipeline.Execute(ctx, row.ID); err != nil {
		// Don't swallow — but also don't 500 if the failure was a legit
		// lifecycle stop (e.g. illegal transition). Re-load the row so the
		// caller sees the post-pipeline status.
		latest, lerr := h.getRequestByID(ctx, row.ID)
		if lerr != nil {
			return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
				"pipeline execute: %v (and reload failed: %v)", err, lerr)
		}
		// Return the row even on pipeline failure — the request exists; the
		// failure is reflected in its status. The connect error code stays
		// non-2xx so clients can branch.
		return connect.NewResponse(&lifecyclev1.CreateRequestResponse{
			Request:       dbRequestToProto(&latest),
			CorrelationId: latest.CorrelationID,
		}), nil
	}

	latest, err := h.getRequestByID(ctx, row.ID)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"reload request after pipeline: %v", err)
	}
	return connect.NewResponse(&lifecyclev1.CreateRequestResponse{
		Request:       dbRequestToProto(&latest),
		CorrelationId: latest.CorrelationID,
	}), nil
}

// GetRequest loads a single request by id.
func (h *LifecycleHandler) GetRequest(
	ctx context.Context,
	req *connect.Request[lifecyclev1.GetRequestRequest],
) (*connect.Response[lifecyclev1.GetRequestResponse], error) {
	id, err := parseSnowflake("request_id", req.Msg.RequestId)
	if err != nil {
		return nil, err
	}
	row, err := h.getRequestByID(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal("request", req.Msg.RequestId, err)
	}
	return connect.NewResponse(&lifecyclev1.GetRequestResponse{
		Request:       dbRequestToProto(&row),
		CorrelationId: row.CorrelationID,
	}), nil
}

// ListRequests returns requests filtered by the (optional) caller-supplied
// criteria. Phase 1 pagination is limit/offset on page_token (an opaque cursor
// holding the offset).
func (h *LifecycleHandler) ListRequests(
	ctx context.Context,
	req *connect.Request[lifecyclev1.ListRequestsRequest],
) (*connect.Response[lifecyclev1.ListRequestsResponse], error) {
	in := req.Msg
	pageSize := int(in.PageSize)
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := parsePageToken(in.PageToken)

	// Build a dynamic WHERE. We use placeholder args in $1..$N order.
	var conds []string
	var args []any
	addCond := func(sql string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(sql, len(args)))
	}
	if in.RequesterId != "" {
		addCond("requester_id = $%d", in.RequesterId)
	}
	if in.TeamId != "" {
		teamID, err := parseSnowflake("team_id", in.TeamId)
		if err != nil {
			return nil, err
		}
		addCond("team_id = $%d", teamID)
	}
	if in.CatalogItemId != "" {
		cid, err := parseSnowflake("catalog_item_id", in.CatalogItemId)
		if err != nil {
			return nil, err
		}
		addCond("catalog_item_id = $%d", cid)
	}
	if len(in.StatusFilter) > 0 {
		statuses := make([]string, 0, len(in.StatusFilter))
		for _, s := range in.StatusFilter {
			if db := requestStatusToDB(s); db != "" {
				statuses = append(statuses, db)
			}
		}
		if len(statuses) > 0 {
			args = append(args, statuses)
			conds = append(conds, fmt.Sprintf("status = ANY($%d)", len(args)))
		}
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	// Args for LIMIT/OFFSET appended after WHERE args.
	args = append(args, pageSize, offset)
	sql := `SELECT ` + requestColumns + ` FROM requests` + where +
		` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(len(args)-1) +
		` OFFSET $` + strconv.Itoa(len(args))

	rows, err := h.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list requests: %v", err)
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[generated.Request])
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"collect requests: %v", err)
	}

	out := make([]*lifecyclev1.LifecycleRequest, 0, len(items))
	for i := range items {
		out = append(out, dbRequestToProto(&items[i]))
	}

	// Build next page token only when we filled the page.
	nextToken := ""
	if len(items) == pageSize {
		nextToken = strconv.Itoa(offset + pageSize)
	}

	return connect.NewResponse(&lifecyclev1.ListRequestsResponse{
		Requests:      out,
		NextPageToken: nextToken,
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// ListRequestEvents returns the append-only event trail for one request.
func (h *LifecycleHandler) ListRequestEvents(
	ctx context.Context,
	req *connect.Request[lifecyclev1.ListRequestEventsRequest],
) (*connect.Response[lifecyclev1.ListRequestEventsResponse], error) {
	in := req.Msg
	id, err := parseSnowflake("request_id", in.RequestId)
	if err != nil {
		return nil, err
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 100
	}

	sql := `SELECT id, request_id, event_type, stage, from_status, to_status,
			actor_id, actor_type, message, correlation_id, occurred_at
		FROM request_events WHERE request_id = $1
		ORDER BY occurred_at ASC LIMIT $2`
	rows, err := h.pool.Query(ctx, sql, id, pageSize)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list request_events: %v", err)
	}
	defer rows.Close()
	type evRow struct {
		ID            int64     `db:"id"`
		RequestID     int64     `db:"request_id"`
		EventType     string    `db:"event_type"`
		Stage         string    `db:"stage"`
		FromStatus    *string   `db:"from_status"`
		ToStatus      *string   `db:"to_status"`
		ActorID       string    `db:"actor_id"`
		ActorType     string    `db:"actor_type"`
		Message       string    `db:"message"`
		CorrelationID string    `db:"correlation_id"`
		OccurredAt    time.Time `db:"occurred_at"`
	}
	evs, err := pgx.CollectRows(rows, pgx.RowToStructByName[evRow])
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"collect request_events: %v", err)
	}

	out := make([]*lifecyclev1.LifecycleEvent, 0, len(evs))
	for _, e := range evs {
		out = append(out, &lifecyclev1.LifecycleEvent{
			Id:            strconv.FormatInt(e.ID, 10),
			RequestId:     strconv.FormatInt(e.RequestID, 10),
			EventType:     e.EventType,
			OccurredAt:    e.OccurredAt.UTC().Format(time.RFC3339Nano),
			CorrelationId: e.CorrelationID,
			FromStatus:    derefStr(e.FromStatus),
			ToStatus:      derefStr(e.ToStatus),
			Stage:         e.Stage,
			Message:       e.Message,
			Actor: &commonv1.Actor{
				UserId: e.ActorID,
				Type:   actorTypeToProto(e.ActorType),
			},
		})
	}
	return connect.NewResponse(&lifecyclev1.ListRequestEventsResponse{
		Events:        out,
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// CancelRequest transitions a request to cancelled via the state machine.
func (h *LifecycleHandler) CancelRequest(
	ctx context.Context,
	req *connect.Request[lifecyclev1.CancelRequestRequest],
) (*connect.Response[lifecyclev1.CancelRequestResponse], error) {
	in := req.Msg
	id, err := parseSnowflake("request_id", in.RequestId)
	if err != nil {
		return nil, err
	}
	row, err := h.getRequestByID(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal("request", in.RequestId, err)
	}

	if orchestrator.IsTerminal(row.Status) {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_ILLEGAL_STATE_TRANSITION, connect.CodeFailedPrecondition,
			"request %d is terminal (%s), cannot cancel", id, row.Status)
	}
	// Optimistic lock: client must pass the version it read.
	if in.ExpectedVersion != 0 && in.ExpectedVersion != row.Version {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, connect.CodeAborted,
			"version mismatch: client=%d server=%d (reload and retry)", in.ExpectedVersion, row.Version)
	}

	next, err := orchestrator.Transition(row.Status, orchestrator.CancelEvent)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_ILLEGAL_STATE_TRANSITION, connect.CodeFailedPrecondition,
			"cancel from %s: %v", row.Status, err)
	}
	if err := h.updateRequestStatus(ctx, id, next, row.Version); err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"update status: %v", err)
	}
	if err := h.logEvent(ctx, id, &row.Status, &next, "human", auth.SubjectFromContext(ctx),
		"cancel", in.Reason); err != nil {
		// Logging failure must not roll back the transition.
		_ = err
	}

	latest, err := h.getRequestByID(ctx, id)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"reload after cancel: %v", err)
	}
	return connect.NewResponse(&lifecyclev1.CancelRequestResponse{
		Request:       dbRequestToProto(&latest),
		CorrelationId: latest.CorrelationID,
	}), nil
}

// ============================================================
// --- Plan + Artifact ---
// ============================================================

// StartPlan resumes the Pipeline from the request's current status. Phase 1
// Pipeline auto-drives through to plan_ready / pending_approval, so this is
// effectively "kick the pipeline again" (e.g. after a retry).
func (h *LifecycleHandler) StartPlan(
	ctx context.Context,
	req *connect.Request[lifecyclev1.StartPlanRequest],
) (*connect.Response[lifecyclev1.StartPlanResponse], error) {
	id, err := parseSnowflake("request_id", req.Msg.RequestId)
	if err != nil {
		return nil, err
	}
	if err := h.pipeline.Execute(ctx, id); err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_TERRAMATE_EXECUTION_FAILED, connect.CodeInternal,
			"pipeline execute: %v", err)
	}
	latest, err := h.getRequestByID(ctx, id)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"reload after start plan: %v", err)
	}
	return connect.NewResponse(&lifecyclev1.StartPlanResponse{
		Request:       dbRequestToProto(&latest),
		CorrelationId: latest.CorrelationID,
	}), nil
}

// GetArtifact loads a plan_artifacts row by id.
func (h *LifecycleHandler) GetArtifact(
	ctx context.Context,
	req *connect.Request[lifecyclev1.GetArtifactRequest],
) (*connect.Response[lifecyclev1.GetArtifactResponse], error) {
	id, err := parseSnowflake("artifact_id", req.Msg.ArtifactId)
	if err != nil {
		return nil, err
	}
	const sql = `SELECT id, request_id, status, plan_hash, storage_uri, sha256, size_bytes,
		pinned_commit, toolchain_profile_hash, provider_lock_hash, tf_version_sha256,
		stack_id, state_key, resources_to_add, resources_to_change, resources_to_destroy,
		cost_estimate_cents, expires_at, created_at
		FROM plan_artifacts WHERE id = $1`
	var p generated.PlanArtifact
	err = h.pool.QueryRow(ctx, sql, id).Scan(
		&p.ID, &p.RequestID, &p.Status, &p.PlanHash, &p.StorageUri, &p.Sha256, &p.SizeBytes,
		&p.PinnedCommit, &p.ToolchainProfileHash, &p.ProviderLockHash, &p.TfVersionSha256,
		&p.StackID, &p.StateKey, &p.ResourcesToAdd, &p.ResourcesToChange, &p.ResourcesToDestroy,
		&p.CostEstimateCents, &p.ExpiresAt, &p.CreatedAt,
	)
	if err != nil {
		return nil, notFoundOrInternal("artifact", req.Msg.ArtifactId, err)
	}
	return connect.NewResponse(&lifecyclev1.GetArtifactResponse{
		Artifact:      dbArtifactToProto(&p),
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// ============================================================
// --- Gate ---
// ============================================================

// EvaluateGate is a Phase 1 no-op: there are no policy gates wired yet, so
// every request passes. Phase 2 (W4) will plug in OPA/conft.
func (h *LifecycleHandler) EvaluateGate(
	ctx context.Context,
	req *connect.Request[lifecyclev1.EvaluateGateRequest],
) (*connect.Response[lifecyclev1.EvaluateGateResponse], error) {
	return connect.NewResponse(&lifecyclev1.EvaluateGateResponse{
		RequestId:     req.Msg.RequestId,
		Passed:        true,
		Gates:         nil, // Phase 1: no gates configured
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// ============================================================
// --- Approval ---
// ============================================================

// ListPendingApprovals returns approval_runs with status='pending'. Phase 1
// ignores approver_id routing (the caller's role_bindings) and returns all
// pending runs — the RBAC layer is responsible for limiting who can call this.
func (h *LifecycleHandler) ListPendingApprovals(
	ctx context.Context,
	req *connect.Request[lifecyclev1.ListPendingApprovalsRequest],
) (*connect.Response[lifecyclev1.ListPendingApprovalsResponse], error) {
	in := req.Msg
	pageSize := int(in.PageSize)
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	var conds []string
	var args []any
	addCond := func(sql string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(sql, len(args)))
	}
	addCond("status = $%d", "pending")
	if in.TeamId != "" {
		// approval_runs has no team_id; join via requests. To keep this Phase 1
		// query simple we filter by request.team_id through a sub-select.
		teamID, err := parseSnowflake("team_id", in.TeamId)
		if err != nil {
			return nil, err
		}
		args = append(args, teamID)
		conds = append(conds, fmt.Sprintf("request_id IN (SELECT id FROM requests WHERE team_id = $%d)", len(args)))
	}
	if in.GateFilter != commonv1.ApprovalGate_APPROVAL_GATE_UNSPECIFIED {
		addCond("gate = $%d", approvalGateToDB(in.GateFilter))
	}

	args = append(args, pageSize)
	sql := `SELECT id, request_id, flow_id, gate, current_node, status, decided_by,
			decided_at, started_at, finished_at, expires_at, version
		FROM approval_runs WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY started_at ASC LIMIT $` + strconv.Itoa(len(args))

	rows, err := h.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list pending approvals: %v", err)
	}
	defer rows.Close()
	type runRow struct {
		ID          int64      `db:"id"`
		RequestID   int64      `db:"request_id"`
		FlowID      int64      `db:"flow_id"`
		Gate        string     `db:"gate"`
		CurrentNode string     `db:"current_node"`
		Status      string     `db:"status"`
		DecidedBy   string     `db:"decided_by"`
		DecidedAt   *time.Time `db:"decided_at"`
		StartedAt   time.Time  `db:"started_at"`
		FinishedAt  *time.Time `db:"finished_at"`
		ExpiresAt   *time.Time `db:"expires_at"`
		Version     int32      `db:"version"`
	}
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByName[runRow])
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"collect approval runs: %v", err)
	}

	out := make([]*lifecyclev1.ApprovalRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, dbApprovalRunToProto(r.ID, r.RequestID, r.Gate, r.CurrentNode, r.Status,
			r.DecidedBy, r.DecidedAt, r.StartedAt, r.FinishedAt, r.ExpiresAt, r.Version))
	}
	return connect.NewResponse(&lifecyclev1.ListPendingApprovalsResponse{
		Runs:          out,
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// GetApprovalRun loads one approval_run plus its node chain and decisions.
func (h *LifecycleHandler) GetApprovalRun(
	ctx context.Context,
	req *connect.Request[lifecyclev1.GetApprovalRunRequest],
) (*connect.Response[lifecyclev1.GetApprovalRunResponse], error) {
	runID, err := parseSnowflake("run_id", req.Msg.RunId)
	if err != nil {
		return nil, err
	}

	const runSQL = `SELECT id, request_id, flow_id, gate, current_node, status, decided_by,
		decided_at, started_at, finished_at, expires_at, version
		FROM approval_runs WHERE id = $1`
	var r struct {
		ID          int64      `db:"id"`
		RequestID   int64      `db:"request_id"`
		FlowID      int64      `db:"flow_id"`
		Gate        string     `db:"gate"`
		CurrentNode string     `db:"current_node"`
		Status      string     `db:"status"`
		DecidedBy   string     `db:"decided_by"`
		DecidedAt   *time.Time `db:"decided_at"`
		StartedAt   time.Time  `db:"started_at"`
		FinishedAt  *time.Time `db:"finished_at"`
		ExpiresAt   *time.Time `db:"expires_at"`
		Version     int32      `db:"version"`
	}
	if err := h.pool.QueryRow(ctx, runSQL, runID).Scan(
		&r.ID, &r.RequestID, &r.FlowID, &r.Gate, &r.CurrentNode, &r.Status, &r.DecidedBy,
		&r.DecidedAt, &r.StartedAt, &r.FinishedAt, &r.ExpiresAt, &r.Version,
	); err != nil {
		return nil, notFoundOrInternal("approval_run", req.Msg.RunId, err)
	}

	// Nodes.
	nodeRows, err := h.pool.Query(ctx,
		`SELECT node_id, mode, decided_count, required_count, status, timeout_at
		 FROM approval_node_runs WHERE run_id = $1 ORDER BY node_id`, runID)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list node runs: %v", err)
	}
	defer nodeRows.Close()
	type nodeRow struct {
		NodeID        string     `db:"node_id"`
		Mode          string     `db:"mode"`
		DecidedCount  int32      `db:"decided_count"`
		RequiredCount int32      `db:"required_count"`
		Status        string     `db:"status"`
		TimeoutAt     *time.Time `db:"timeout_at"`
	}
	nodes, _ := pgx.CollectRows(nodeRows, pgx.RowToStructByName[nodeRow])
	outNodes := make([]*lifecyclev1.ApprovalNodeRun, 0, len(nodes))
	for _, n := range nodes {
		outNodes = append(outNodes, &lifecyclev1.ApprovalNodeRun{
			NodeId:        n.NodeID,
			Mode:          nodeModeToProto(n.Mode),
			DecidedCount:  n.DecidedCount,
			RequiredCount: n.RequiredCount,
			Status:        nodeStatusToProto(n.Status),
		})
	}

	// Decisions (joined through node_runs to scope by run).
	decRows, err := h.pool.Query(ctx,
		`SELECT d.approver_id, d.decision, d.comment, d.decided_at
		 FROM approval_decisions d
		 JOIN approval_node_runs n ON n.id = d.node_run_id
		 WHERE n.run_id = $1
		 ORDER BY d.decided_at ASC`, runID)
	if err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
			"list decisions: %v", err)
	}
	defer decRows.Close()
	type decRow struct {
		ApproverID string     `db:"approver_id"`
		Decision   string     `db:"decision"`
		Comment    string     `db:"comment"`
		DecidedAt  *time.Time `db:"decided_at"`
	}
	decs, _ := pgx.CollectRows(decRows, pgx.RowToStructByName[decRow])
	outDecs := make([]*lifecyclev1.ApprovalDecisionRecord, 0, len(decs))
	for _, d := range decs {
		outDecs = append(outDecs, &lifecyclev1.ApprovalDecisionRecord{
			ApproverId: d.ApproverID,
			Decision:   decisionToProto(d.Decision),
			Comment:    d.Comment,
		})
	}

	return connect.NewResponse(&lifecyclev1.GetApprovalRunResponse{
		Run: dbApprovalRunToProto(r.ID, r.RequestID, r.Gate, r.CurrentNode, r.Status, r.DecidedBy,
			r.DecidedAt, r.StartedAt, r.FinishedAt, r.ExpiresAt, r.Version),
		Nodes:         outNodes,
		Decisions:     outDecs,
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// DecideApproval maps a human decision onto ApprovalService.Approve / Reject
// (W2-06, per P0-4 — no new Engine). expected_run_version is used to guard
// against concurrent decisions; we read the run to validate it.
func (h *LifecycleHandler) DecideApproval(
	ctx context.Context,
	req *connect.Request[lifecyclev1.DecideApprovalRequest],
) (*connect.Response[lifecyclev1.DecideApprovalResponse], error) {
	in := req.Msg
	runID, err := parseSnowflake("run_id", in.RunId)
	if err != nil {
		return nil, err
	}
	approver := auth.SubjectFromContext(ctx)

	// Load the run to get request_id + version for optimistic locking.
	var (
		requestID int64
		status    string
		version   int32
	)
	err = h.pool.QueryRow(ctx,
		`SELECT request_id, status, version FROM approval_runs WHERE id = $1`, runID,
	).Scan(&requestID, &status, &version)
	if err != nil {
		return nil, notFoundOrInternal("approval_run", in.RunId, err)
	}
	if status != "pending" {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_ILLEGAL_STATE_TRANSITION, connect.CodeFailedPrecondition,
			"approval run %d is %s (not pending)", runID, status)
	}
	if in.ExpectedRunVersion != 0 && in.ExpectedRunVersion != version {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, connect.CodeAborted,
			"run version mismatch: client=%d server=%d (reload and retry)", in.ExpectedRunVersion, version)
	}

	switch in.Decision {
	case commonv1.ApprovalDecision_APPROVAL_DECISION_APPROVED:
		if err := h.approval.Approve(ctx, requestID, approver); err != nil {
			return nil, mapApprovalErr(err)
		}
	case commonv1.ApprovalDecision_APPROVAL_DECISION_REJECTED:
		if err := h.approval.Reject(ctx, requestID, approver, in.Comment); err != nil {
			return nil, mapApprovalErr(err)
		}
	default:
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_SCHEMA_INVALID, connect.CodeInvalidArgument,
			"decision must be APPROVED or REJECTED, got %v", in.Decision)
	}

	// Reload the run for the response (status + decided_by updated by the
	// approval service via the request status; for Phase 1 we surface the
	// run row as-is plus a fresh status read).
	var (
		gate        string
		currentNode string
		decidedBy   string
		decidedAt   *time.Time
		startedAt   time.Time
		finishedAt  *time.Time
		expiresAt   *time.Time
	)
	_ = h.pool.QueryRow(ctx,
		`SELECT gate, current_node, status, decided_by, decided_at, started_at, finished_at, expires_at, version
		 FROM approval_runs WHERE id = $1`, runID,
	).Scan(&gate, &currentNode, &status, &decidedBy, &decidedAt, &startedAt, &finishedAt, &expiresAt, &version)

	return connect.NewResponse(&lifecyclev1.DecideApprovalResponse{
		Run: dbApprovalRunToProto(runID, requestID, gate, currentNode, status, decidedBy,
			decidedAt, startedAt, finishedAt, expiresAt, version),
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// ============================================================
// --- Apply ---
// ============================================================

// StartApply resumes the Pipeline from applying. DecideApproval has already
// flipped the request to applying; this RPC is the "go" button.
func (h *LifecycleHandler) StartApply(
	ctx context.Context,
	req *connect.Request[lifecyclev1.StartApplyRequest],
) (*connect.Response[lifecyclev1.StartApplyResponse], error) {
	id, err := parseSnowflake("request_id", req.Msg.RequestId)
	if err != nil {
		return nil, err
	}
	if err := h.pipeline.Execute(ctx, id); err != nil {
		return nil, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_TERRAMATE_EXECUTION_FAILED, connect.CodeInternal,
			"pipeline execute (apply): %v", err)
	}
	return connect.NewResponse(&lifecyclev1.StartApplyResponse{
		CorrelationId: correlationIDFrom(ctx),
	}), nil
}

// ============================================================
// Helpers: persistence
// ============================================================

// requestColumns is the canonical SELECT column list for requests, ordered to
// match generated.Request field order so pgx.RowToStructByName works.
const requestColumns = `id, catalog_item_id, space_id, env_id, tenant_id, team_id, requester_id,
	kind, source, status, current_stage, form_values_json, form_hash,
	resolved_params_json, idempotency_key, pinned_commit, plan_artifact_id,
	cost_estimate_cents, cost_currency, correlation_id, retry_count, version,
	layer_rule_set_version_id, created_at, updated_at`

// insertRequest writes a requests row. The idempotency_key UNIQUE index makes
// concurrent CreateRequest calls collide-safe (one wins, the other gets a
// UNIQUE violation and the replay path returns the existing row).
func (h *LifecycleHandler) insertRequest(ctx context.Context, r generated.Request) error {
	const sql = `INSERT INTO requests
		(id, catalog_item_id, space_id, env_id, tenant_id, team_id, requester_id,
		 kind, source, status, current_stage, form_values_json, form_hash,
		 idempotency_key, cost_estimate_cents, cost_currency, correlation_id, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`
	_, err := h.pool.Exec(ctx, sql,
		r.ID, r.CatalogItemID, r.SpaceID, r.EnvID, r.TenantID, r.TeamID, r.RequesterID,
		r.Kind, r.Source, r.Status, r.CurrentStage, r.FormValuesJson, r.FormHash,
		r.IdempotencyKey, r.CostEstimateCents, r.CostCurrency, r.CorrelationID, r.Version)
	return err
}

// getRequestByID loads a request by snowflake id.
func (h *LifecycleHandler) getRequestByID(ctx context.Context, id int64) (generated.Request, error) {
	sql := `SELECT ` + requestColumns + ` FROM requests WHERE id = $1`
	rows, err := h.pool.Query(ctx, sql, id)
	if err != nil {
		return generated.Request{}, err
	}
	defer rows.Close()
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[generated.Request])
	if err != nil {
		return generated.Request{}, err
	}
	return r, nil
}

// getRequestByIdempotencyKey returns the previously-inserted request for an
// idempotency replay. ok=false when none exists.
func (h *LifecycleHandler) getRequestByIdempotencyKey(ctx context.Context, key string) (generated.Request, bool, error) {
	if key == "" {
		return generated.Request{}, false, nil
	}
	sql := `SELECT ` + requestColumns + ` FROM requests WHERE idempotency_key = $1`
	rows, err := h.pool.Query(ctx, sql, key)
	if err != nil {
		return generated.Request{}, false, err
	}
	defer rows.Close()
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[generated.Request])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return generated.Request{}, false, nil
		}
		return generated.Request{}, false, err
	}
	return r, true, nil
}

// updateRequestStatus flips status with the optimistic-lock guard.
func (h *LifecycleHandler) updateRequestStatus(ctx context.Context, id int64, status string, expectedVersion int32) error {
	const sql = `UPDATE requests SET status = $1, version = version + 1
		WHERE id = $2 AND version = $3`
	tag, err := h.pool.Exec(ctx, sql, status, id, expectedVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either the row is gone or the version changed under us.
		return pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, connect.CodeAborted,
			"optimistic lock failed for request %d at version %d", id, expectedVersion)
	}
	return nil
}

// logEvent appends a request_events row.
func (h *LifecycleHandler) logEvent(
	ctx context.Context,
	requestID int64,
	fromStatus, toStatus *string,
	actorType, actorID, stage, message string,
) error {
	const sql = `INSERT INTO request_events
		(id, request_id, event_type, stage, from_status, to_status, actor_id, actor_type, message, correlation_id)
		VALUES ($1, $2, 'state_transition', $3, $4, $5, $6, $7, $8, $9)`
	_, err := h.pool.Exec(ctx, sql,
		utils.GenerateID(), requestID, stage, fromStatus, toStatus, actorID, actorType, message,
		correlationIDFrom(ctx))
	return err
}

// ============================================================
// Helpers: proto ↔ DB conversion
// ============================================================

// dbRequestToProto maps a requests row to the proto LifecycleRequest. int64
// IDs are stringified per the proto contract.
func dbRequestToProto(r *generated.Request) *lifecyclev1.LifecycleRequest {
	out := &lifecyclev1.LifecycleRequest{
		Id:                strconv.FormatInt(r.ID, 10),
		Status:            requestStatusToProto(r.Status),
		CatalogItemId:     strconv.FormatInt(r.CatalogItemID, 10),
		EnvId:             r.EnvID,
		TeamId:            strconv.FormatInt(r.TeamID, 10),
		TenantId:          r.TenantID,
		FormHash:          r.FormHash,
		Source:            sourceDBToProto(r.Source),
		CostEstimateCents: r.CostEstimateCents,
		CostCurrency:      r.CostCurrency,
		Version:           r.Version,
		CorrelationId:     r.CorrelationID,
		RequesterId:       r.RequesterID,
		CurrentStage:      r.CurrentStage,
	}
	if r.SpaceID != nil {
		out.SpaceId = strconv.FormatInt(*r.SpaceID, 10)
	}
	if r.PinnedCommit != nil {
		out.PinnedCommit = *r.PinnedCommit
	}
	if r.PlanArtifactID != nil {
		out.PlanArtifactId = strconv.FormatInt(*r.PlanArtifactID, 10)
	}
	if len(r.FormValuesJson) > 0 {
		_ = json.Unmarshal(r.FormValuesJson, &out.FormValues)
	}
	if r.CreatedAt.Valid {
		out.CreatedAt = r.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if r.UpdatedAt.Valid {
		out.UpdatedAt = r.UpdatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// dbArtifactToProto maps a plan_artifacts row.
func dbArtifactToProto(p *generated.PlanArtifact) *lifecyclev1.PlanArtifact {
	out := &lifecyclev1.PlanArtifact{
		Id:                   strconv.FormatInt(p.ID, 10),
		RequestId:            strconv.FormatInt(p.RequestID, 10),
		Status:               artifactStatusToProto(p.Status),
		PlanHash:             p.PlanHash,
		StorageUri:           p.StorageUri,
		Sha256:               p.Sha256,
		PinnedCommit:         p.PinnedCommit,
		ToolchainProfileHash: p.ToolchainProfileHash,
		ProviderLockHash:     p.ProviderLockHash,
		TfVersionSha256:      p.TfVersionSha256,
		StackId:              p.StackID,
		StateKey:             p.StateKey,
		SizeBytes:            p.SizeBytes,
		CostEstimateCents:    p.CostEstimateCents,
		Summary: &lifecyclev1.PlanSummary{
			ResourcesToAdd:     p.ResourcesToAdd,
			ResourcesToChange:  p.ResourcesToChange,
			ResourcesToDestroy: p.ResourcesToDestroy,
		},
	}
	if p.ExpiresAt.Valid {
		out.ExpiresAt = p.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if p.CreatedAt.Valid {
		out.CreatedAt = p.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// dbApprovalRunToProto assembles an ApprovalRun proto from raw column values.
// The intermediate loose typing keeps the callers (ListPending / Get / Decide)
// short — they all just read columns.
func dbApprovalRunToProto(
	id, requestID int64,
	gate, currentNode, status, decidedBy string,
	decidedAt *time.Time,
	startedAt time.Time,
	finishedAt, expiresAt *time.Time,
	version int32,
) *lifecyclev1.ApprovalRun {
	out := &lifecyclev1.ApprovalRun{
		RunId:       strconv.FormatInt(id, 10),
		Status:      approvalRunStatusToProto(status),
		DecidedBy:   decidedBy,
		RequestId:   strconv.FormatInt(requestID, 10),
		Gate:        approvalGateFromDB(gate),
		CurrentNode: currentNode,
		Version:     version,
	}
	if !startedAt.IsZero() {
		out.StartedAt = startedAt.UTC().Format(time.RFC3339Nano)
	}
	if decidedAt != nil {
		out.DecidedAt = decidedAt.UTC().Format(time.RFC3339Nano)
	}
	if finishedAt != nil {
		out.FinishedAt = finishedAt.UTC().Format(time.RFC3339Nano)
	}
	if expiresAt != nil {
		out.ExpiresAt = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// ============================================================
// Helpers: enum conversion (proto <-> DB string)
// ============================================================

func requestStatusToProto(s string) commonv1.RequestStatus {
	switch s {
	case orchestrator.StatusSubmitted:
		return commonv1.RequestStatus_REQUEST_STATUS_SUBMITTED
	case orchestrator.StatusGenerating:
		return commonv1.RequestStatus_REQUEST_STATUS_GENERATING
	case orchestrator.StatusPendingAdmission:
		return commonv1.RequestStatus_REQUEST_STATUS_PENDING_ADMISSION
	case orchestrator.StatusPlanning:
		return commonv1.RequestStatus_REQUEST_STATUS_PLANNING
	case orchestrator.StatusPlanReady:
		return commonv1.RequestStatus_REQUEST_STATUS_PLAN_READY
	case orchestrator.StatusPendingApproval:
		return commonv1.RequestStatus_REQUEST_STATUS_PENDING_APPROVAL
	case orchestrator.StatusApplying:
		return commonv1.RequestStatus_REQUEST_STATUS_APPLYING
	case orchestrator.StatusReconciling:
		return commonv1.RequestStatus_REQUEST_STATUS_RECONCILING
	case orchestrator.StatusSucceeded:
		return commonv1.RequestStatus_REQUEST_STATUS_SUCCEEDED
	case orchestrator.StatusReconcilePending:
		return commonv1.RequestStatus_REQUEST_STATUS_RECONCILE_PENDING
	case orchestrator.StatusRejected:
		return commonv1.RequestStatus_REQUEST_STATUS_REJECTED
	case orchestrator.StatusCancelled:
		return commonv1.RequestStatus_REQUEST_STATUS_CANCELLED
	case orchestrator.StatusExpired:
		return commonv1.RequestStatus_REQUEST_STATUS_EXPIRED
	case orchestrator.StatusFailedRetryable:
		return commonv1.RequestStatus_REQUEST_STATUS_FAILED_RETRYABLE
	case orchestrator.StatusFailedTerminal:
		return commonv1.RequestStatus_REQUEST_STATUS_FAILED_TERMINAL
	case orchestrator.StatusWaitingManual:
		return commonv1.RequestStatus_REQUEST_STATUS_WAITING_MANUAL
	case orchestrator.StatusBlockedPolicy:
		return commonv1.RequestStatus_REQUEST_STATUS_BLOCKED_POLICY
	case orchestrator.StatusBlockedStateHealth:
		return commonv1.RequestStatus_REQUEST_STATUS_BLOCKED_STATE_HEALTH
	case orchestrator.StatusPausedDrift:
		return commonv1.RequestStatus_REQUEST_STATUS_PAUSED_DRIFT
	}
	return commonv1.RequestStatus_REQUEST_STATUS_UNSPECIFIED
}

// requestStatusToDB inverts requestStatusToProto; "" means "filter out".
func requestStatusToDB(s commonv1.RequestStatus) string {
	switch s {
	case commonv1.RequestStatus_REQUEST_STATUS_SUBMITTED:
		return orchestrator.StatusSubmitted
	case commonv1.RequestStatus_REQUEST_STATUS_GENERATING:
		return orchestrator.StatusGenerating
	case commonv1.RequestStatus_REQUEST_STATUS_PENDING_ADMISSION:
		return orchestrator.StatusPendingAdmission
	case commonv1.RequestStatus_REQUEST_STATUS_PLANNING:
		return orchestrator.StatusPlanning
	case commonv1.RequestStatus_REQUEST_STATUS_PLAN_READY:
		return orchestrator.StatusPlanReady
	case commonv1.RequestStatus_REQUEST_STATUS_PENDING_APPROVAL:
		return orchestrator.StatusPendingApproval
	case commonv1.RequestStatus_REQUEST_STATUS_APPLYING:
		return orchestrator.StatusApplying
	case commonv1.RequestStatus_REQUEST_STATUS_RECONCILING:
		return orchestrator.StatusReconciling
	case commonv1.RequestStatus_REQUEST_STATUS_SUCCEEDED:
		return orchestrator.StatusSucceeded
	case commonv1.RequestStatus_REQUEST_STATUS_RECONCILE_PENDING:
		return orchestrator.StatusReconcilePending
	case commonv1.RequestStatus_REQUEST_STATUS_REJECTED:
		return orchestrator.StatusRejected
	case commonv1.RequestStatus_REQUEST_STATUS_CANCELLED:
		return orchestrator.StatusCancelled
	case commonv1.RequestStatus_REQUEST_STATUS_EXPIRED:
		return orchestrator.StatusExpired
	case commonv1.RequestStatus_REQUEST_STATUS_FAILED_RETRYABLE:
		return orchestrator.StatusFailedRetryable
	case commonv1.RequestStatus_REQUEST_STATUS_FAILED_TERMINAL:
		return orchestrator.StatusFailedTerminal
	case commonv1.RequestStatus_REQUEST_STATUS_WAITING_MANUAL:
		return orchestrator.StatusWaitingManual
	case commonv1.RequestStatus_REQUEST_STATUS_BLOCKED_POLICY:
		return orchestrator.StatusBlockedPolicy
	case commonv1.RequestStatus_REQUEST_STATUS_BLOCKED_STATE_HEALTH:
		return orchestrator.StatusBlockedStateHealth
	case commonv1.RequestStatus_REQUEST_STATUS_PAUSED_DRIFT:
		return orchestrator.StatusPausedDrift
	}
	return ""
}

func sourceEnumToDB(s commonv1.RequestSource) string {
	switch s {
	case commonv1.RequestSource_REQUEST_SOURCE_WEB:
		return "web"
	case commonv1.RequestSource_REQUEST_SOURCE_CLI:
		return "cli"
	case commonv1.RequestSource_REQUEST_SOURCE_CICD:
		return "cicd"
	case commonv1.RequestSource_REQUEST_SOURCE_AI:
		return "ai"
	case commonv1.RequestSource_REQUEST_SOURCE_GATEWAY:
		return "gateway"
	}
	return "web"
}

func sourceDBToProto(s string) commonv1.RequestSource {
	switch s {
	case "web":
		return commonv1.RequestSource_REQUEST_SOURCE_WEB
	case "cli":
		return commonv1.RequestSource_REQUEST_SOURCE_CLI
	case "cicd":
		return commonv1.RequestSource_REQUEST_SOURCE_CICD
	case "ai":
		return commonv1.RequestSource_REQUEST_SOURCE_AI
	case "gateway":
		return commonv1.RequestSource_REQUEST_SOURCE_GATEWAY
	}
	return commonv1.RequestSource_REQUEST_SOURCE_UNSPECIFIED
}

func artifactStatusToProto(s string) commonv1.ArtifactStatus {
	switch s {
	case "ready":
		return commonv1.ArtifactStatus_ARTIFACT_STATUS_READY
	case "expired":
		return commonv1.ArtifactStatus_ARTIFACT_STATUS_EXPIRED
	case "consumed":
		return commonv1.ArtifactStatus_ARTIFACT_STATUS_CONSUMED
	case "superseded":
		return commonv1.ArtifactStatus_ARTIFACT_STATUS_SUPERSEDED
	}
	return commonv1.ArtifactStatus_ARTIFACT_STATUS_UNSPECIFIED
}

func approvalRunStatusToProto(s string) commonv1.ApprovalRunStatus {
	switch s {
	case "pending":
		return commonv1.ApprovalRunStatus_APPROVAL_RUN_STATUS_PENDING
	case "approved":
		return commonv1.ApprovalRunStatus_APPROVAL_RUN_STATUS_APPROVED
	case "rejected":
		return commonv1.ApprovalRunStatus_APPROVAL_RUN_STATUS_REJECTED
	case "expired":
		return commonv1.ApprovalRunStatus_APPROVAL_RUN_STATUS_EXPIRED
	}
	return commonv1.ApprovalRunStatus_APPROVAL_RUN_STATUS_UNSPECIFIED
}

func approvalGateFromDB(s string) commonv1.ApprovalGate {
	switch s {
	case "pre_plan":
		return commonv1.ApprovalGate_APPROVAL_GATE_PRE_PLAN
	case "pre_apply":
		return commonv1.ApprovalGate_APPROVAL_GATE_PRE_APPLY
	case "break_glass_retroactive":
		return commonv1.ApprovalGate_APPROVAL_GATE_BREAK_GLASS_RETROACTIVE
	}
	return commonv1.ApprovalGate_APPROVAL_GATE_UNSPECIFIED
}

func approvalGateToDB(g commonv1.ApprovalGate) string {
	switch g {
	case commonv1.ApprovalGate_APPROVAL_GATE_PRE_PLAN:
		return "pre_plan"
	case commonv1.ApprovalGate_APPROVAL_GATE_PRE_APPLY:
		return "pre_apply"
	case commonv1.ApprovalGate_APPROVAL_GATE_BREAK_GLASS_RETROACTIVE:
		return "break_glass_retroactive"
	}
	return ""
}

func nodeModeToProto(s string) commonv1.ApprovalNodeMode {
	switch s {
	case "any":
		return commonv1.ApprovalNodeMode_APPROVAL_NODE_MODE_ANY
	case "all":
		return commonv1.ApprovalNodeMode_APPROVAL_NODE_MODE_ALL
	case "majority":
		return commonv1.ApprovalNodeMode_APPROVAL_NODE_MODE_MAJORITY
	case "quorum":
		return commonv1.ApprovalNodeMode_APPROVAL_NODE_MODE_QUORUM
	}
	return commonv1.ApprovalNodeMode_APPROVAL_NODE_MODE_UNSPECIFIED
}

func nodeStatusToProto(s string) commonv1.ApprovalNodeStatus {
	switch s {
	case "pending":
		return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_PENDING
	case "approved":
		return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_APPROVED
	case "rejected":
		return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_REJECTED
	case "skipped":
		return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_SKIPPED
	case "timeout":
		return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_TIMEOUT
	}
	return commonv1.ApprovalNodeStatus_APPROVAL_NODE_STATUS_UNSPECIFIED
}

func decisionToProto(s string) commonv1.ApprovalDecision {
	switch s {
	case "approved":
		return commonv1.ApprovalDecision_APPROVAL_DECISION_APPROVED
	case "rejected":
		return commonv1.ApprovalDecision_APPROVAL_DECISION_REJECTED
	}
	return commonv1.ApprovalDecision_APPROVAL_DECISION_UNSPECIFIED
}

func actorTypeToProto(s string) commonv1.ActorType {
	switch s {
	case "human":
		return commonv1.ActorType_ACTOR_TYPE_HUMAN
	case "ai":
		return commonv1.ActorType_ACTOR_TYPE_AI
	case "system":
		return commonv1.ActorType_ACTOR_TYPE_SYSTEM
	}
	return commonv1.ActorType_ACTOR_TYPE_UNSPECIFIED
}

// ============================================================
// Helpers: misc
// ============================================================

// parseSnowflake parses a proto string id into int64, returning a structured
// INVALID_ARGUMENT error on failure.
func parseSnowflake(field, v string) (int64, error) {
	if v == "" {
		return 0, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_SCHEMA_INVALID, connect.CodeInvalidArgument,
			"%s is required", field)
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_SCHEMA_INVALID, connect.CodeInvalidArgument,
			"%s must be a numeric snowflake id, got %q", field, v)
	}
	return id, nil
}

// notFoundOrInternal maps a pgx.ErrNoRows to NOT_FOUND and anything else to
// INTERNAL. entity is the human label; id is the caller-provided id.
func notFoundOrInternal(entity, id string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_REQUEST_NOT_FOUND, connect.CodeNotFound,
			"%s %q not found", entity, id)
	}
	return pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
		"load %s %q: %v", entity, id, err)
}

// mapApprovalErr converts orchestrator approval errors into structured codes.
func mapApprovalErr(err error) error {
	if errors.Is(err, orchestrator.ErrNotPendingApproval) {
		return pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_ILLEGAL_STATE_TRANSITION, connect.CodeFailedPrecondition,
			"%v", err)
	}
	return pkgerrors.New(commonv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR, connect.CodeInternal,
		"approval decision: %v", err)
}

// buildIdempotencyKey computes sha256(subject || catalog || form_hash || day)
// per P1-5. Day buckets to UTC midnight so a replay within the same UTC day
// returns the original request.
func buildIdempotencyKey(subject, catalogItemID, formHash string) string {
	day := time.Now().UTC().Format("2006-01-02")
	h := sha256.Sum256([]byte(subject + "|" + catalogItemID + "|" + formHash + "|" + day))
	return hex.EncodeToString(h[:])
}

// encodeFormValues marshals the proto map and computes its sha256 hash.
func encodeFormValues(m map[string]string) ([]byte, string, error) {
	if m == nil {
		m = map[string]string{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

// correlationIDFrom reads the correlation id from ctx (set by the Connect
// Auth interceptor via events.WithCorrelationID). Falls back to "" when no
// correlation id is in flight (the DB default is "").
func correlationIDFrom(ctx context.Context) string {
	return events.CorrelationIDFromContext(ctx)
}

// parsePageToken parses an opaque offset-based cursor. "" → 0.
func parsePageToken(t string) int {
	if t == "" {
		return 0
	}
	n, err := strconv.Atoi(t)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// derefStr returns *s or "".
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FormatTimeUTC formats a time for proto string fields.
func FormatTimeUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// Ensure pkgx pool import is used even when scanning helper is unused.
var _ = fmt.Sprintf
