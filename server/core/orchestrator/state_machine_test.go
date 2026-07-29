package orchestrator

import (
	"errors"
	"testing"
)

// TestTransition_Legal covers every transition explicitly listed in
// design.md D1, plus the two global rules (cancel / manual_intervention) for
// at least one representative non-terminal state. Each row is one cell of the
// transition matrix — if the matrix regresses, exactly one row fails.
func TestTransition_Legal(t *testing.T) {
	cases := []struct {
		name    string
		current string
		event   Event
		want    string
	}{
		// --- Explicit happy-path + failure rules (design D1, in order) ---
		{"submit starts generation", StatusSubmitted, SubmitEvent, StatusGenerating},
		{"gen_done enters planning", StatusGenerating, GenDoneEvent, StatusPlanning},
		{"gen_fail is retryable", StatusGenerating, GenFailEvent, StatusFailedRetryable},
		{"plan_done is ready for approval", StatusPlanning, PlanDoneEvent, StatusPlanReady},
		{"plan_fail is retryable", StatusPlanning, PlanFailEvent, StatusFailedRetryable},
		{"plan_ready requests approval", StatusPlanReady, RequestApprovalEvent, StatusPendingApproval},
		{"approve starts applying", StatusPendingApproval, ApproveEvent, StatusApplying},
		{"reject is terminal", StatusPendingApproval, RejectEvent, StatusRejected},
		{"approval timeout expires", StatusPendingApproval, ApprovalTimeoutEvent, StatusExpired},
		{"apply_done enters reconcile", StatusApplying, ApplyDoneEvent, StatusReconciling},
		{"apply transient fail is retryable", StatusApplying, ApplyTransientFailEvent, StatusFailedRetryable},
		{"apply permanent fail is terminal", StatusApplying, ApplyPermanentFailEvent, StatusFailedTerminal},
		{"apply interrupted waits for human", StatusApplying, ApplyInterruptedEvent, StatusWaitingManual},
		{"reconcile_done succeeds", StatusReconciling, ReconcileDoneEvent, StatusSucceeded},

		// --- Reserved-state forward rules ---
		{"reconcile_pending also succeeds", StatusReconcilePending, ReconcileDoneEvent, StatusSucceeded},
		{"failed_retryable retries to generating", StatusFailedRetryable, RetryEvent, StatusGenerating},
		{"waiting_manual resumes to applying", StatusWaitingManual, ResumeEvent, StatusApplying},

		// --- Global rule: cancel from any non-terminal state ---
		{"cancel from planning", StatusPlanning, CancelEvent, StatusCancelled},
		{"cancel from pending_approval", StatusPendingApproval, CancelEvent, StatusCancelled},
		{"cancel from applying", StatusApplying, CancelEvent, StatusCancelled},
		{"cancel from failed_retryable", StatusFailedRetryable, CancelEvent, StatusCancelled},
		{"cancel from reserved blocked_policy", StatusBlockedPolicy, CancelEvent, StatusCancelled},

		// --- Global rule: manual_intervention from any non-terminal state ---
		{"manual intervention from planning", StatusPlanning, ManualInterventionEvent, StatusWaitingManual},
		{"manual intervention from applying", StatusApplying, ManualInterventionEvent, StatusWaitingManual},
		{"manual intervention from expired", StatusExpired, ManualInterventionEvent, StatusWaitingManual},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Transition(tc.current, tc.event)
			if err != nil {
				t.Fatalf("Transition(%q, %s) returned unexpected error: %v", tc.current, tc.event, err)
			}
			if got != tc.want {
				t.Errorf("Transition(%q, %s) = %q, want %q", tc.current, tc.event, got, tc.want)
			}
		})
	}
}

// TestTransition_Illegal covers the negative space: terminal states reject
// everything, and unrelated (current, event) pairs error. This guards against
// accidentally widening the matrix.
func TestTransition_Illegal(t *testing.T) {
	cases := []struct {
		name    string
		current string
		event   Event
	}{
		// --- Terminal states: no outgoing transitions at all ---
		{"succeeded cannot approve", StatusSucceeded, ApproveEvent},
		{"succeeded cannot cancel", StatusSucceeded, CancelEvent},
		{"rejected cannot retry", StatusRejected, RetryEvent},
		{"cancelled cannot resume", StatusCancelled, ResumeEvent},
		{"failed_terminal cannot retry", StatusFailedTerminal, RetryEvent},

		// --- Wrong event for a non-terminal state ---
		{"submitted cannot approve (no approval yet)", StatusSubmitted, ApproveEvent},
		{"plan_ready cannot apply (needs approval first)", StatusPlanReady, ApplyDoneEvent},
		{"generating cannot apply", StatusGenerating, ApplyDoneEvent},
		{"pending_approval cannot plan_done", StatusPendingApproval, PlanDoneEvent},
		{"applying cannot gen_done", StatusApplying, GenDoneEvent},

		// --- Unknown state ---
		{"unknown state errors", "not_a_real_status", SubmitEvent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Transition(tc.current, tc.event)
			if err == nil {
				t.Fatalf("Transition(%q, %s) = %q, want error", tc.current, tc.event, got)
			}
			var ile *ErrIllegalTransition
			if !errors.As(err, &ile) {
				t.Fatalf("Transition(%q, %s) returned %T, want *ErrIllegalTransition", tc.current, tc.event, err)
			}
			if ile.Current != tc.current || ile.Event != tc.event {
				t.Errorf("ErrIllegalTransition fields = (%q,%s), want (%q,%s)",
					ile.Current, ile.Event, tc.current, tc.event)
			}
		})
	}
}

// TestIsTerminal pins the four terminal statuses and confirms a few
// non-terminal ones are correctly reported as movable.
func TestIsTerminal(t *testing.T) {
	terminal := []string{StatusSucceeded, StatusRejected, StatusCancelled, StatusFailedTerminal}
	for _, s := range terminal {
		if !IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = false, want true", s)
		}
	}
	nonTerminal := []string{
		StatusSubmitted, StatusGenerating, StatusPlanning, StatusPlanReady,
		StatusPendingApproval, StatusApplying, StatusReconciling,
		StatusFailedRetryable, StatusWaitingManual, StatusExpired,
		StatusReconcilePending, StatusBlockedPolicy, StatusBlockedStateHealth,
		StatusPausedDrift, StatusPendingAdmission,
	}
	for _, s := range nonTerminal {
		if IsTerminal(s) {
			t.Errorf("IsTerminal(%q) = true, want false", s)
		}
	}
}
