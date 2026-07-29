// Package orchestrator implements the W2 request lifecycle engine (design
// openspec/changes/w2-orchestrator/design.md). It has three concerns:
//
//  1. StateMachine — a PURE transition function over the 19-status lifecycle
//     (D1). No DB, no side effects: (currentStatus, event) -> (newStatus, err).
//  2. Pipeline — drives a request through its stages by injecting interfaces
//     for codegen / terramate / workspace / store (D2, D4). Phase 1 runs the
//     stages synchronously in-process.
//  3. ApprovalService — Phase 1 simplified single-gate or-approval (D3): an
//     approve/reject API that does NOT go through the approval_flows DSL.
//
// Every transition is audited via EventLogger (D5) into request_events.
package orchestrator

import (
	"fmt"
)

// Event is a lifecycle input that drives a state transition. Event values are
// NOT stored in the DB (only the resulting status is); they are an in-process
// vocabulary kept lowercase-with-underscores to read alongside the status
// strings. See design.md D1 for the transition matrix.
type Event string

const (
	// Happy-path / forward events.
	SubmitEvent             Event = "submit"
	GenDoneEvent            Event = "gen_done"
	GenFailEvent            Event = "gen_fail"
	PlanDoneEvent           Event = "plan_done"
	PlanFailEvent           Event = "plan_fail"
	RequestApprovalEvent    Event = "request_approval"
	ApproveEvent            Event = "approve"
	RejectEvent             Event = "reject"
	ApprovalTimeoutEvent    Event = "timeout"
	ApplyDoneEvent          Event = "apply_done"
	ApplyTransientFailEvent Event = "apply_transient_fail"
	ApplyPermanentFailEvent Event = "apply_permanent_fail"
	ApplyInterruptedEvent   Event = "apply_interrupted"
	ReconcileDoneEvent      Event = "reconcile_done"
	CancelEvent             Event = "cancel"
	ManualInterventionEvent Event = "manual_intervention"
	// Retry/resume events for reserved states (design "missing branches"):
	// failed_retryable can retry into generating or planning depending on where
	// it failed; waiting_manual can resume back into applying or generating.
	// Phase 1 keeps these in the matrix so callers can drive them; the Pipeline
	// only executes the main 8 happy-path stages.
	RetryEvent  Event = "retry"
	ResumeEvent Event = "resume"
)

// Request lifecycle statuses. These EXACT underscore strings are the values
// stored in requests.status (migration 005 CHECK constraint — 19 values). Do
// NOT change casing or separators: the DB will reject unknown values.
const (
	StatusSubmitted          = "submitted"
	StatusGenerating         = "generating"
	StatusPendingAdmission   = "pending_admission"
	StatusPlanning           = "planning"
	StatusPlanReady          = "plan_ready"
	StatusPendingApproval    = "pending_approval"
	StatusApplying           = "applying"
	StatusReconciling        = "reconciling"
	StatusSucceeded          = "succeeded"
	StatusReconcilePending   = "reconcile_pending"
	StatusRejected           = "rejected"
	StatusCancelled          = "cancelled"
	StatusExpired            = "expired"
	StatusFailedRetryable    = "failed_retryable"
	StatusFailedTerminal     = "failed_terminal"
	StatusWaitingManual      = "waiting_manual"
	StatusBlockedPolicy      = "blocked_policy"
	StatusBlockedStateHealth = "blocked_state_health"
	StatusPausedDrift        = "paused_drift"
)

// terminalStates are the statuses with no outgoing transitions. Once a request
// reaches one of these, the lifecycle is finished (success or permanent
// failure). Everything else can still move (including the "reserved" states
// blocked_policy / blocked_state_health / paused_drift / reconcile_pending /
// expired / waiting_manual / failed_retryable, which can resume via the
// appropriate event).
var terminalStates = map[string]struct{}{
	StatusSucceeded:      {},
	StatusRejected:       {},
	StatusCancelled:      {},
	StatusFailedTerminal: {},
}

// IsTerminal reports whether status has no legal outgoing transitions.
func IsTerminal(status string) bool {
	_, ok := terminalStates[status]
	return ok
}

// transitions is the transition matrix: transitions[current][event] = next.
//
// Global rules (design D1):
//   - cancel is valid from ANY non-terminal state.
//   - manual_intervention is valid from any non-terminal state EXCEPT
//     succeeded/cancelled (which are terminal) — i.e. every non-terminal
//     state can be forced into waiting_manual by a human operator.
//
// Build() composes the explicit per-state rules with these two global rules
// so the table stays readable in this source file.
var transitions = buildTransitionMatrix()

// buildTransitionMatrix constructs the transition map at init time. It starts
// from the explicit happy-path + failure table and then layers the two global
// rules (cancel / manual_intervention) onto every non-terminal state.
func buildTransitionMatrix() map[string]map[Event]string {
	// Explicit rules — copied verbatim from design.md D1. Each inner map is
	// per-state; the global rules are added afterward to avoid repetition.
	explicit := map[string]map[Event]string{
		StatusSubmitted: {
			SubmitEvent: StatusGenerating,
		},
		StatusGenerating: {
			GenDoneEvent: StatusPlanning,
			GenFailEvent: StatusFailedRetryable,
		},
		StatusPlanning: {
			PlanDoneEvent: StatusPlanReady,
			PlanFailEvent: StatusFailedRetryable,
		},
		StatusPlanReady: {
			RequestApprovalEvent: StatusPendingApproval,
		},
		StatusPendingApproval: {
			ApproveEvent:         StatusApplying,
			RejectEvent:          StatusRejected,
			ApprovalTimeoutEvent: StatusExpired,
		},
		StatusApplying: {
			ApplyDoneEvent:          StatusReconciling,
			ApplyTransientFailEvent: StatusFailedRetryable,
			ApplyPermanentFailEvent: StatusFailedTerminal,
			ApplyInterruptedEvent:   StatusWaitingManual,
		},
		StatusReconciling: {
			ReconcileDoneEvent: StatusSucceeded,
		},
		// Reserved states (design "missing branches", Phase 1 matrix-only):
		// reconcile_pending -> succeeded once the CMDB/state ingester catches up.
		StatusReconcilePending: {
			ReconcileDoneEvent: StatusSucceeded,
		},
		// failed_retryable can retry. Phase 1 Pipeline retries into generating
		// by default; if the failure was during planning, callers transition
		// through generating first (the Pipeline re-runs codegen, which is
		// idempotent and cheap). Both targets are reachable here so callers
		// that pin a specific retry point can use them.
		StatusFailedRetryable: {
			RetryEvent: StatusGenerating,
		},
		// waiting_manual resumes by re-entering the stage it was interrupted
		// in. Default target is applying (the most common interruption point);
		// callers may also resume into generating if the human re-ran codegen.
		StatusWaitingManual: {
			ResumeEvent: StatusApplying,
		},
	}

	// Layer the two global rules onto every non-terminal state. cancel goes to
	// cancelled; manual_intervention goes to waiting_manual. We do NOT clobber
	// an explicit entry (e.g. pending_approval+reject already routes to
	// rejected, not via the global rule) — globals only fill in gaps.
	for state := range explicit {
		if IsTerminal(state) {
			continue
		}
		if _, set := explicit[state][CancelEvent]; !set {
			explicit[state][CancelEvent] = StatusCancelled
		}
		if _, set := explicit[state][ManualInterventionEvent]; !set {
			explicit[state][ManualInterventionEvent] = StatusWaitingManual
		}
	}

	// Non-terminal "reserved" states that have no explicit forward rule still
	// need the global cancel / manual_intervention entries. Create maps for
	// them so cancel works from anywhere non-terminal.
	for _, s := range []string{
		StatusPendingAdmission,
		StatusBlockedPolicy,
		StatusBlockedStateHealth,
		StatusPausedDrift,
		StatusExpired,
	} {
		if explicit[s] == nil {
			explicit[s] = map[Event]string{}
		}
		explicit[s][CancelEvent] = StatusCancelled
		explicit[s][ManualInterventionEvent] = StatusWaitingManual
	}

	return explicit
}

// ErrIllegalTransition is returned by Transition for any (current, event) pair
// that is not in the matrix. The error carries both inputs so callers can log
// a precise audit trail without re-threading them through stack frames.
type ErrIllegalTransition struct {
	Current string
	Event   Event
}

func (e *ErrIllegalTransition) Error() string {
	return fmt.Sprintf("orchestrator: illegal transition %q + %s", e.Current, e.Event)
}

// Transition is the pure lifecycle transition function (design D1).
//
// Given the current status and an event, it returns the new status or an error
// if the move is not allowed. It performs NO I/O and mutates NO state, which
// makes it trivially table-testable and safe to call from any goroutine.
//
// Terminal states (succeeded / rejected / cancelled / failed_terminal) reject
// every event — they have no outgoing transitions at all.
func Transition(current string, event Event) (string, error) {
	events, ok := transitions[current]
	if !ok {
		return "", &ErrIllegalTransition{Current: current, Event: event}
	}
	next, ok := events[event]
	if !ok {
		return "", &ErrIllegalTransition{Current: current, Event: event}
	}
	return next, nil
}
