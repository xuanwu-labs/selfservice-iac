package orchestrator

import (
	"context"
	"errors"
	"fmt"
)

// ApprovalDecisionRecorder persists a single approver decision. In Phase 2
// this writes approval_decisions tied to an approval_runs / node_runs graph
// (migration 007). Phase 1 (design D3) skips the DSL and just records the
// decision directly — the value type below mirrors the approval_decisions
// row shape so the data layer can insert without translation.
type ApprovalDecisionRecorder interface {
	RecordApprovalDecision(ctx context.Context, requestID int64, approverID, decision, comment string) error
}

// ApprovalDecisionValue is the canonical value stored in
// approval_decisions.decision (migration 007 CHECK constraint).
const (
	ApprovalDecisionApproved = "approved"
	ApprovalDecisionRejected = "rejected"
)

// ErrNotPendingApproval is returned when Approve/Reject is called on a request
// that is not in the pending_approval state. Phase 1 approval is a single
// pre-apply gate: only requests sitting at pending_approval can be decided.
var ErrNotPendingApproval = errors.New("orchestrator: request is not pending approval")

// ApprovalService implements Phase 1 simplified or-approval (design D3): any
// one approver can approve or reject, and the call flips the request to
// applying (approve) or rejected (reject). Multi-approver AND / quorum /
// timeout / multi-gate are Phase 2 (approval_flows DSL, W4 task 12).
type ApprovalService struct {
	store     RequestStore
	events    EventLogger
	decisions ApprovalDecisionRecorder
}

// NewApprovalService constructs an ApprovalService. decisions may be nil for
// Phase 1 deployments that record decisions only via request_events (the
// service treats a nil recorder as a no-op). store and events are required.
func NewApprovalService(store RequestStore, events EventLogger, decisions ApprovalDecisionRecorder) *ApprovalService {
	return &ApprovalService{store: store, events: events, decisions: decisions}
}

// Approve records an approval decision and transitions the request from
// pending_approval to applying. After this call returns, the request is ready
// for the Pipeline to run the apply stage.
func (s *ApprovalService) Approve(ctx context.Context, requestID int64, approverID string) error {
	return s.decide(ctx, requestID, approverID, ApprovalDecisionApproved, "", ApproveEvent, StatusApplying)
}

// Reject records a rejection decision (with an optional reason) and
// transitions the request from pending_approval to rejected (terminal).
func (s *ApprovalService) Reject(ctx context.Context, requestID int64, approverID string, reason string) error {
	return s.decide(ctx, requestID, approverID, ApprovalDecisionRejected, reason, RejectEvent, StatusRejected)
}

// decide is the shared Approve/Reject body. It:
//  1. Loads the request and guards that it is pending_approval.
//  2. Records the approval decision (Phase 1: a single row; Phase 2 will be
//     a node_run decision instead).
//  3. Transitions via the pure Transition function and persists + logs.
//
// The decision is recorded BEFORE the state transition so an audit trail
// exists even if the transition write fails. If the transition fails after
// the decision is recorded, the caller sees the error and can retry the
// transition without re-recording (Phase 2 will make RecordApprovalDecision
// idempotent on (node_run_id, approver_id) — IDEMP-004).
func (s *ApprovalService) decide(ctx context.Context, requestID int64, approverID, decision, comment string, event Event, wantNext string) error {
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("orchestrator: get request %d: %w", requestID, err)
	}
	if req.Status != StatusPendingApproval {
		return fmt.Errorf("orchestrator: request %d status=%q: %w", requestID, req.Status, ErrNotPendingApproval)
	}

	// Record the human decision. Phase 1: directly to approval_decisions via
	// the recorder; a nil recorder means "events-only" mode.
	if s.decisions != nil {
		if err := s.decisions.RecordApprovalDecision(ctx, requestID, approverID, decision, comment); err != nil {
			return fmt.Errorf("orchestrator: record approval decision: %w", err)
		}
	}

	// Compute the transition via the pure function. This is belt-and-suspenders
	// — pending_approval + approve/reject are in the matrix — but it gives a
	// single source of truth and a clear error if the matrix regresses.
	next, err := Transition(req.Status, event)
	if err != nil {
		return fmt.Errorf("orchestrator: transition %q + %s: %w", req.Status, event, err)
	}
	if next != wantNext {
		// Matrix drifted from the decision's expectation. Refuse to apply a
		// surprising transition; surface as a programmer-visible error.
		return fmt.Errorf("orchestrator: approval transition yielded %q, want %q (matrix regression)", next, wantNext)
	}

	if err := s.store.UpdateStatus(ctx, requestID, next, req.Version); err != nil {
		return fmt.Errorf("orchestrator: update status %d %q->%q: %w", requestID, req.Status, next, err)
	}

	if err := s.events.LogEvent(ctx, requestID, req.Status, next, approverID, map[string]any{
		"event":       string(event),
		"decision":    decision,
		"approver_id": approverID,
		"comment":     comment,
		"version":     req.Version,
	}); err != nil {
		return fmt.Errorf("orchestrator: log approval event %d: %w", requestID, err)
	}
	return nil
}
