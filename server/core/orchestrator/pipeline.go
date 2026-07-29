package orchestrator

import (
	"context"
	"fmt"

	"github.com/xuanwu-labs/selfservice-iac/server/core/codegen"
	"github.com/xuanwu-labs/selfservice-iac/server/pkg/db/generated"
)

// Pipeline collaborators are defined HERE as local interfaces (Dependency
// Inversion Principle, design D2). The concrete implementations in other
// packages satisfy them implicitly — we never import those packages' concrete
// types, only their input/output types where those are shared value types.
//
// This keeps the orchestrator testable with fakes (see pipeline_test.go) and
// decouples it from the eventual real workspace manager / river job runner.

// CodeGenerator generates a stack's file tree from a CodegenInput. Satisfied
// by *codegen.Generator (W2-05). We use codegen's own CodegenInput / FileSet
// types rather than redefining them, since those are pure value types shared
// across the codebase.
type CodeGenerator interface {
	Generate(ctx context.Context, input codegen.CodegenInput) (codegen.FileSet, error)
}

// PlanResult is the orchestrator's view of a `terraform plan` (run via
// terramate). It carries the metrics that drive downstream cost / resource
// accounting and the path to the saved plan file used by RunApplySavedPlan.
type PlanResult struct {
	PlanFile           string // path to the saved binary plan, passed to RunApplySavedPlan
	PlanHash           string // sha256 of the plan content (stored on plan_artifacts)
	ResourcesToAdd     int32
	ResourcesToChange  int32
	ResourcesToDestroy int32
	CostEstimateCents  int64
	Stdout             string
}

// ApplyResult is the orchestrator's view of a `terraform apply` of a saved
// plan. ExitCode 0 means success; the orchestrator transitions to reconciling.
type ApplyResult struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMs int64
}

// TerramateRunner runs plan / apply via terramate in a stack directory. This
// is a HIGHER-level interface than core/terramate.Adapter (which only exposes
// a generic Run) — the orchestrator speaks in plan/apply terms so the matrix
// in state_machine.go maps directly onto method outcomes. A real adapter will
// wrap terramate.Adapter.Run to produce these typed results.
type TerramateRunner interface {
	RunPlan(ctx context.Context, dir string, args []string) (PlanResult, error)
	RunApplySavedPlan(ctx context.Context, dir string, planFile string) (ApplyResult, error)
}

// WorkspaceManager persists a generated FileSet to git and returns the commit
// SHA the request is pinned to. Phase 1 uses a stub (design D2 note); W2-07
// replaces it with the real go-git worktree implementation.
type WorkspaceManager interface {
	WriteFiles(ctx context.Context, requestID int64, files codegen.FileSet) (commitSHA string, err error)
}

// Request is the orchestrator-local alias for the sqlc-generated Request row.
// Aliasing keeps function signatures short and lets us swap the row type later
// without churn across this package.
type Request = generated.Request

// RequestStore is the persistence interface the Pipeline needs: read the
// current row (status + version + form values) and update status with
// optimistic locking (version is the lock token — design D1 / migration 005
// "version INTEGER ... optimistic lock").
type RequestStore interface {
	GetRequest(ctx context.Context, id int64) (Request, error)
	UpdateStatus(ctx context.Context, id int64, status string, version int32) error
}

// EventLogger appends a request_events row for every state transition (D5).
// The real implementation writes to the request_events table; tests use the
// in-memory recorder in events.go.
type EventLogger interface {
	LogEvent(ctx context.Context, requestID int64, from, to string, actor string, context map[string]any) error
}

// Pipeline drives a single request through its lifecycle stages (design D2,
// D4). It is constructed once (via wire) and called per request. Each call to
// Execute resumes from the request's current status and runs forward until it
// hits a wait point (plan_ready awaits approval; reconciling awaits success)
// or a terminal/error state.
type Pipeline struct {
	codegen   CodeGenerator
	terramate TerramateRunner
	workspace WorkspaceManager
	store     RequestStore
	events    EventLogger
}

// NewPipeline constructs a Pipeline from its injected collaborators. All five
// are required — passing nil will compile but panic at runtime on first use,
// which is preferable to silent no-ops for an orchestration core.
func NewPipeline(cg CodeGenerator, tr TerramateRunner, ws WorkspaceManager, store RequestStore, events EventLogger) *Pipeline {
	return &Pipeline{
		codegen:   cg,
		terramate: tr,
		workspace: ws,
		store:     store,
		events:    events,
	}
}

// Execute reads the request's current status and runs the corresponding stage.
// It loops forward across stages (submitted -> generating -> planning ->
// plan_ready) until it reaches a wait point or a terminal state, then returns.
//
// Stage outcomes map to events via Transition (state_machine.go); on any
// execution failure the request is moved to the appropriate failure state and
// the error is returned to the caller (the executor / job runner) so it can
// surface the failure and schedule a retry if the new state is retryable.
//
// Phase 1 (design D4): runs synchronously. A context timeout per stage should
// be applied by the caller (open question in design.md); we honour ctx here.
func (p *Pipeline) Execute(ctx context.Context, requestID int64) error {
	for {
		req, err := p.store.GetRequest(ctx, requestID)
		if err != nil {
			return fmt.Errorf("orchestrator: get request %d: %w", requestID, err)
		}

		switch req.Status {
		case StatusSubmitted:
			// submit -> generating, then fall through to the generating stage
			// in the next loop iteration.
			if err := p.advance(ctx, req, SubmitEvent, "pipeline"); err != nil {
				return err
			}
			continue

		case StatusGenerating:
			if err := p.runGenerating(ctx, req); err != nil {
				return err
			}
			continue

		case StatusPlanning:
			if err := p.runPlanning(ctx, req); err != nil {
				return err
			}
			continue

		case StatusPlanReady:
			// Hand off to the approval subsystem; Execute returns and waits.
			return p.advance(ctx, req, RequestApprovalEvent, "pipeline")

		case StatusPendingApproval:
			// Awaiting a human decision — nothing for the Pipeline to do.
			return nil

		case StatusApplying:
			if err := p.runApplying(ctx, req); err != nil {
				return err
			}
			continue

		case StatusReconciling:
			// Phase 1: reconcile is a no-op marker; the apply already
			// committed state. Close out to succeeded.
			return p.advance(ctx, req, ReconcileDoneEvent, "pipeline")

		case StatusSucceeded, StatusRejected, StatusCancelled, StatusFailedTerminal:
			// Terminal — nothing to do.
			return nil

		default:
			// Reserved / out-of-band states (failed_retryable, waiting_manual,
			// blocked_*, paused_drift, expired, reconcile_pending). Phase 1
			// Pipeline does not auto-drive these; an external action (retry,
			// resume, reconciler) is expected to move them first.
			return nil
		}
	}
}

// runGenerating is the generating stage: codegen produces a FileSet, the
// workspace manager commits it, and the request advances to planning.
// Failure at either step -> failed_retryable (gen_fail).
func (p *Pipeline) runGenerating(ctx context.Context, req Request) error {
	// NOTE: building the CodegenInput from the request row (resolving catalog
	// defaults, governance vars, backend config, etc.) is the orchestrator's
	// job. Phase 1 stub: an empty input is enough to drive codegen in tests;
	// W2-08 wires the full resolver. We do NOT fail here on empty input —
	// codegen is deterministic and the contract is that the caller fills it.
	input := codegen.CodegenInput{}

	files, err := p.codegen.Generate(ctx, input)
	if err != nil {
		return p.fail(ctx, req, GenFailEvent, "codegen", fmt.Errorf("generate: %w", err))
	}

	commitSHA, err := p.workspace.WriteFiles(ctx, req.ID, files)
	if err != nil {
		// git commit failure is retryable (transient: lock contention, network).
		return p.fail(ctx, req, GenFailEvent, "workspace", fmt.Errorf("write files: %w", err))
	}
	_ = commitSHA // Phase 1: persisted by the workspace manager; W2-07 pins it on the request.

	if err := p.advance(ctx, req, GenDoneEvent, "pipeline"); err != nil {
		return err
	}
	return nil
}

// runPlanning is the planning stage: terramate runs `terraform plan` in the
// stack dir. On success -> plan_ready; on failure -> failed_retryable.
func (p *Pipeline) runPlanning(ctx context.Context, req Request) error {
	// dir + args are resolved by the workspace manager in W2-07; Phase 1 uses
	// the request ID as a placeholder working dir. Plan artifacts (storage_uri,
	// hashes) are persisted by W2-08.
	dir := workDir(req.ID)
	result, err := p.terramate.RunPlan(ctx, dir, nil)
	if err != nil {
		return p.fail(ctx, req, PlanFailEvent, "terramate", fmt.Errorf("run plan: %w", err))
	}
	_ = result // Phase 1: W2-08 persists PlanResult into plan_artifacts.

	if err := p.advance(ctx, req, PlanDoneEvent, "pipeline"); err != nil {
		return err
	}
	return nil
}

// runApplying is the apply stage: terramate runs `terraform apply` against the
// saved plan. On success -> reconciling; on transient failure ->
// failed_retryable; on permanent failure -> failed_terminal; on interruption
// (context cancel / executor heartbeat loss) -> waiting_manual.
func (p *Pipeline) runApplying(ctx context.Context, req Request) error {
	dir := workDir(req.ID)
	// planFile would come from plan_artifacts (W2-08). Empty string is fine for
	// the Phase 1 stub/fakes; the real terramate adapter would reject it.
	planFile := ""
	result, err := p.terramate.RunApplySavedPlan(ctx, dir, planFile)
	if err != nil {
		// Context cancellation means the executor lost the handle (heartbeat
		// loss / shutdown) — route to waiting_manual so a human or a resume
		// picks it up, rather than burning a retry.
		if ctx.Err() != nil {
			return p.fail(ctx, req, ApplyInterruptedEvent, "executor", fmt.Errorf("apply interrupted: %w", err))
		}
		// Heuristic: a non-zero exit code with a recoverable shape (timeout /
		// 429) is transient; anything else is treated as permanent for Phase 1.
		// W2-08 refines this with provider error classification.
		evt := ApplyTransientFailEvent
		if isPermanentApplyFailure(result, err) {
			evt = ApplyPermanentFailEvent
		}
		return p.fail(ctx, req, evt, "terramate", fmt.Errorf("run apply: %w", err))
	}

	if err := p.advance(ctx, req, ApplyDoneEvent, "pipeline"); err != nil {
		return err
	}
	return nil
}

// advance performs a single state transition end-to-end: compute the new
// status via the pure Transition function, persist it (optimistic lock on
// version), and log a request_events row. actor is who/what triggered the
// move ("pipeline", "approval", a user id, ...).
func (p *Pipeline) advance(ctx context.Context, req Request, event Event, actor string) error {
	next, err := Transition(req.Status, event)
	if err != nil {
		// A legal stage should never produce an illegal transition — this is a
		// programmer error (e.g. stage dispatched from the wrong status).
		return fmt.Errorf("orchestrator: transition %q + %s: %w", req.Status, event, err)
	}
	if err := p.store.UpdateStatus(ctx, req.ID, next, req.Version); err != nil {
		return fmt.Errorf("orchestrator: update status %d %q->%q: %w", req.ID, req.Status, next, err)
	}
	if err := p.events.LogEvent(ctx, req.ID, req.Status, next, actor, map[string]any{
		"event":   string(event),
		"version": req.Version,
	}); err != nil {
		// Logging failure must not roll back the transition — the state change
		// already happened. Surface it so the caller can alert, but the
		// request is already advanced.
		return fmt.Errorf("orchestrator: log event %d %q->%q: %w", req.ID, req.Status, next, err)
	}
	return nil
}

// fail transitions the request to a failure state and returns a wrapped error
// describing the original failure. The failure event (gen_fail / plan_fail /
// apply_*) selects the target state via Transition; the returned error lets
// the caller decide whether to retry, alert, or give up.
func (p *Pipeline) fail(ctx context.Context, req Request, event Event, actor string, cause error) error {
	if advErr := p.advance(ctx, req, event, actor); advErr != nil {
		// advance already logged/persisted what it could; wrap both errors so
		// neither the original cause nor the advance failure is hidden.
		return fmt.Errorf("orchestrator: status=%q event=%s cause=%v; advance failed: %w",
			req.Status, event, cause, advErr)
	}
	return fmt.Errorf("orchestrator: request %d %q via %s: %w", req.ID, req.Status, event, cause)
}

// isPermanentApplyFailure classifies an apply error as permanent (config /
// permissions / syntax — won't succeed on retry) vs transient (rate limit /
// timeout / network). Phase 1 heuristic: an empty / nil ApplyResult with a
// non-context error is treated as permanent; a populated result with a non-zero
// exit code is transient (the process ran, it just failed — usually retryable).
// W2-08 replaces this with provider-error classification.
func isPermanentApplyFailure(result ApplyResult, err error) bool {
	// If we got no structured result back, the failure happened before/outside
	// terraform (e.g. binary missing, worktree missing) — treat as permanent
	// so we don't burn retries on a configuration problem.
	return result.ExitCode == 0 && result.Stdout == "" && result.Stderr == "" && err != nil
}

// workDir is the Phase 1 placeholder for a request's working directory. W2-07
// (workspace manager) replaces this with the real worktree path keyed by
// requestID. Centralizing it here means only one place changes.
func workDir(requestID int64) string {
	return fmt.Sprintf("/var/lib/selfservice/worktrees/%d", requestID)
}
